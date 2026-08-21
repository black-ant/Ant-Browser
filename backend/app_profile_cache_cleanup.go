package backend

import (
	"os"
	"path/filepath"

	"ant-chrome/backend/internal/logger"
)

// profileCacheDirs 是可安全删除的纯缓存目录(相对 userDataDir)。
//
// 红线:不得加入 Cookies / Network / Local Storage / IndexedDB / Service Worker / Preferences ——
// 那是账号状态与站点可见存储,删了轻则掉登录,重则形成"缓存被清空"的行为异常面。
// 这里只清 HTTP 缓存、V8 编译缓存与 GPU/着色器缓存,它们对直播场景复用价值≈0。
var profileCacheDirs = []string{
	"Default/Cache",
	"Default/Code Cache",
	"Default/GPUCache",
	"Default/DawnGraphiteCache",
	"Default/DawnWebGPUCache",
	"GrShaderCache",
	"ShaderCache",
	"GraphiteDawnCache",
}

// cleanProfileCaches 删除单个实例数据目录下的纯缓存目录,返回释放字节数。
// 部分目录删除失败不影响其余目录,返回首个错误供调用方记录。
func cleanProfileCaches(userDataDir string) (int64, error) {
	var freed int64
	var firstErr error

	for _, rel := range profileCacheDirs {
		dir := filepath.Join(userDataDir, filepath.FromSlash(rel))
		size := dirSize(dir)
		if size == 0 {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		freed += size
	}
	return freed, firstErr
}

// dirSize 统计目录下所有常规文件的字节数;目录不存在时返回 0。
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// CleanStoppedProfileCaches 对所有未运行实例执行缓存清理,返回释放总字节数(Wails 导出)。
// 只处理未运行实例 —— 运行中实例的缓存文件被 Chromium 占用,删除会失败或引发异常。
func (a *App) CleanStoppedProfileCaches() int64 {
	if a == nil || a.browserMgr == nil {
		return 0
	}
	log := logger.New("CacheClean")
	var total int64

	for _, p := range a.browserMgr.List() {
		if p.Running {
			continue
		}
		profile := p
		freed, err := cleanProfileCaches(a.browserMgr.ResolveUserDataDir(&profile))
		if err != nil {
			log.Warn("实例缓存清理部分失败",
				logger.F("profile_id", p.ProfileId),
				logger.F("error", err.Error()),
			)
		}
		total += freed
	}

	if total > 0 {
		log.Info("已回收未运行实例缓存", logger.F("freed_mb", total/1024/1024))
	}
	return total
}
