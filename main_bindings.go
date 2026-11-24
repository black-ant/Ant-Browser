//go:build bindings

package main

// Wails 用 -tags bindings 编译一个临时程序来生成前端绑定。
// 该模式下 main() 会完整跑到 wails.Run，必须跳过单实例锁：锁目录在用户级（%LOCALAPPDATA%\Ant Browser），
// 只要本机有正常实例在运行，生成程序就会被判定为非主实例直接退出，绑定静默不更新。
const bindingsGenerationMode = true
