//go:build unix

package main

import "syscall"

func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
