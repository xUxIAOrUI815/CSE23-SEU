# CryoLab 构建（在 WSL2 里运行 make）
# 需要：Go（~/.local/go/bin）、cc（build-essential）

NSHELPER ?= nshelper
GO ?= go

all: $(NSHELPER)
	$(GO) build -o cryo ./cmd/cryo

$(NSHELPER): nshelper.c
	cc -O2 -Wall -o $(NSHELPER) nshelper.c

test: $(NSHELPER)
	CRYO_NSHELPER=$(CURDIR)/$(NSHELPER) $(GO) test ./...

clean:
	rm -f $(NSHELPER) cryo

.PHONY: all test clean
