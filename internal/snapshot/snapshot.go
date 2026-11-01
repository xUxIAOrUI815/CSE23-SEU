package snapshot

// Overlay 描述一个 overlayfs 冻结挂载（lowerdir=只读冻结层，upperdir=可丢弃写层）。
// 实际挂载由 C 助手 nshelper 完成（单线程 userns 内 mount）。
type Overlay struct {
	LowerDir   string
	UpperDir   string
	WorkDir    string
	MountPoint string
}
