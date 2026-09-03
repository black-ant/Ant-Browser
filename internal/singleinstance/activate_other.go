//go:build !windows

package singleinstance

func grantExistingSingleInstanceForeground(pid int) {}

func ActivateExistingWindow(pid int) {}
