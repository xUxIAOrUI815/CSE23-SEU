// nshelper — CryoLab 的命名空间/特权设置助手（单线程 C，等价于 runc 的 nsexec）。
//
// 纯 Go 用 clone(CLONE_NEWUSER) 时，子进程在写 uid_map 前后都拿不到有效 capability
// （effective 集因 uid 未映射而空），因此既设不了 securebits 也改不了 bounding set、也挂不了 overlayfs。
// 这个 C 程序在单线程上下文里用 unshare(CLONE_NEWUSER) 成为 userns 的「owner」，写自己的 uid_map 后
// 仍保有完整 capability，从而能完成这些特权操作。
//
// 用法：nshelper <go-binary> __cryo-init <target argv...>
// 环境变量（由 Go 引擎设置并透传）：
//   CRYO_DROP_CAPS=1        降权：securebits NOROOT + 清 bounding set + 清 caps
//   CRYO_OVERLAY_LOWER/UPPER/WORK/MNT  挂载 overlayfs（冻结 + 可丢弃写层）
#define _GNU_SOURCE
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mount.h>
#include <sys/prctl.h>
#include <sys/wait.h>
#include <sys/syscall.h>

#define CAP_LAST 40
#define LINUX_CAPABILITY_VERSION_3 0x20080522

static void die(const char *msg) {
    perror(msg);
    exit(1);
}

static void write_file(const char *path, const char *data) {
    int fd = open(path, O_WRONLY);
    if (fd < 0) die(path);
    if (write(fd, data, strlen(data)) < 0) die("write");
    close(fd);
}

static void write_id_map(uid_t uid, gid_t gid) {
    char buf[64];
    snprintf(buf, sizeof(buf), "0 %d 1\n", (int)uid);
    write_file("/proc/self/uid_map", buf);
    write_file("/proc/self/setgroups", "deny\n");
    snprintf(buf, sizeof(buf), "0 %d 1\n", (int)gid);
    write_file("/proc/self/gid_map", buf);
}

static void drop_caps(void) {
    // SECBIT_NOROOT(1<<0) | SECBIT_NOROOT_LOCKED(1<<1)：禁止 uid 0 在 exec 时重获 capability
    if (prctl(PR_SET_SECUREBITS, (1 << 0) | (1 << 1), 0, 0, 0) < 0) die("securebits");
    for (int i = 0; i <= CAP_LAST; i++) {
        prctl(PR_CAPBSET_DROP, i, 0, 0, 0); // 清 bounding set；EINVAL 忽略
    }
    struct { unsigned int version; int pid; } hdr = { LINUX_CAPABILITY_VERSION_3, 0 };
    struct { unsigned int eff, perm, inh; } data[2] = {{0, 0, 0}, {0, 0, 0}};
    if (syscall(SYS_capset, &hdr, data) < 0) die("capset");
}

static void mount_overlay(const char *lower, const char *upper, const char *work, const char *mnt) {
    char opts[4096];
    snprintf(opts, sizeof(opts), "lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work);
    if (mount("overlay", mnt, "overlay", 0, opts) < 0) die("mount overlay");
}

int main(int argc, char **argv) {
    if (argc < 4) {
        fprintf(stderr, "usage: nshelper <go-binary> __cryo-init <target...>\n");
        return 1;
    }
    const char *go_binary = argv[1];
    uid_t uid = getuid();
    gid_t gid = getgid();

    // 1. user namespace（单线程，成功后成为 userns owner，获得完整 capability）
    if (unshare(CLONE_NEWUSER) < 0) die("unshare user");
    write_id_map(uid, gid);

    // 2. 其余命名空间（此时已有 CAP_SYS_ADMIN）
    if (unshare(CLONE_NEWNS | CLONE_NEWPID | CLONE_NEWIPC | CLONE_NEWUTS | CLONE_NEWNET) < 0)
        die("unshare ns");

    // 把新挂载命名空间设为 slave，防止挂载传播回宿主（runc 同款）
    if (mount("", "/", "", MS_SLAVE | MS_REC, NULL) < 0)
        die("mount slave");

    // 3. fork：PID 命名空间要 fork 后子进程才成为 PID 1
    pid_t pid = fork();
    if (pid < 0) die("fork");
    if (pid > 0) {
        int status;
        waitpid(pid, &status, 0);
        if (WIFEXITED(status)) return WEXITSTATUS(status);
        return 128 + WTERMSIG(status);
    }

    // 4. 子进程（PID 1）：先挂 overlay（需要 CAP_SYS_ADMIN），再降权，最后 exec 回 Go init
    if (getenv("CRYO_OVERLAY_LOWER"))
        mount_overlay(getenv("CRYO_OVERLAY_LOWER"), getenv("CRYO_OVERLAY_UPPER"),
                      getenv("CRYO_OVERLAY_WORK"), getenv("CRYO_OVERLAY_MNT"));
    if (getenv("CRYO_DROP_CAPS")) drop_caps();

    execve(go_binary, argv + 1, environ); // argv+1 = [go-binary, __cryo-init, target...]
    die("execve go init");
}
