// realign-geo:把「活跃实例」的 fingerprint_args 从其结构化身份重新规范化(canonicalize),
// 使其与最新序列化规则一致 —— 关键是补上 `--user-agent`,让 UA 与 Client Hints 报同一版本
// (实现"版本多样性且自洽");同时对「直连(无代理)」实例把地理对齐到本地国家(默认 CN)。
//
// 不改 seed / 平台 / 硬件等设备指纹字段(版本与地理之外一律保持)。设备指纹稳定。
// 必须在主程序已退出(未占用数据库)时运行。默认 dry-run,加 --apply 才写库。
//
// 用法:
//
//	go run ./backend/cmd/realign-geo --db "<app.db 路径>"            # 预览
//	go run ./backend/cmd/realign-geo --db "<app.db 路径>" --apply    # 执行
//	可选:--country CN、--name Test-001(只处理指定实例名)
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"ant-chrome/backend/internal/identity"

	_ "modernc.org/sqlite"
)

func main() {
	var dbPath, country, nameFilter string
	var apply bool
	flag.StringVar(&dbPath, "db", "", "app.db 路径(必填)")
	flag.StringVar(&country, "country", "CN", "直连实例的目标国家码(ISO alpha-2)")
	flag.StringVar(&nameFilter, "name", "", "只处理该实例名(留空=全部活跃实例)")
	flag.BoolVar(&apply, "apply", false, "写库执行(默认仅预览)")
	flag.Parse()

	if strings.TrimSpace(dbPath) == "" {
		fmt.Fprintln(os.Stderr, "错误:必须用 --db 指定 app.db 路径")
		os.Exit(2)
	}
	if _, _, _, ok := identity.CountryDefault(country); !ok {
		fmt.Fprintf(os.Stderr, "错误:未收录的国家码 %q\n", country)
		os.Exit(2)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败:%v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	store := identity.NewSQLiteStore(db)

	// 只处理未软删除的实例(页面同样按 deleted_at 过滤)。
	rows, err := db.Query(`SELECT profile_id, profile_name, proxy_id, proxy_config, fingerprint_args
		FROM browser_profiles WHERE deleted_at IS NULL OR deleted_at = ''`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询实例失败:%v\n", err)
		os.Exit(1)
	}
	type target struct {
		id, name string
		direct   bool
		curArgs  []string
	}
	var targets []target
	for rows.Next() {
		var id, name, proxyID, proxyConfig, fpRaw string
		if err := rows.Scan(&id, &name, &proxyID, &proxyConfig, &fpRaw); err != nil {
			rows.Close()
			fmt.Fprintf(os.Stderr, "读取行失败:%v\n", err)
			os.Exit(1)
		}
		if nameFilter != "" && name != nameFilter {
			continue
		}
		var cur []string
		_ = json.Unmarshal([]byte(fpRaw), &cur)
		targets = append(targets, target{id: id, name: name, direct: isDirect(proxyID, proxyConfig), curArgs: cur})
	}
	rows.Close()

	if len(targets) == 0 {
		fmt.Println("没有匹配的活跃实例。")
		return
	}

	fmt.Printf("模式:%s  直连目标国家:%s  活跃实例:%d\n\n", modeLabel(apply), country, len(targets))
	changed := 0
	for _, tg := range targets {
		id, ok, err := store.Load(tg.id)
		if err != nil {
			fmt.Printf("  [跳过] %-22s 身份读取失败:%v\n", tg.name, err)
			continue
		}
		if !ok {
			id = identity.FromLaunchArgs(tg.curArgs) // 老环境:从 flag 反解
		}
		if tg.direct {
			id = identity.AlignToCountry(id, country) // 直连:对齐本地地理(幂等)
		}
		newArgs := id.LaunchArgs()

		if sameMultiset(tg.curArgs, newArgs) {
			continue // 已规范,无需改动
		}
		changed++
		fmt.Printf("  [规范化] %-22s %s%s\n", tg.name, diffUA(tg.curArgs, newArgs), diffTZ(tg.curArgs, newArgs))

		if !apply {
			continue
		}
		if ok { // 有结构化身份才回存(对齐可能改了 tz/locale)
			if err := store.Save(tg.id, id); err != nil {
				fmt.Printf("            写身份失败:%v\n", err)
				continue
			}
		}
		fpJSON, _ := json.Marshal(newArgs)
		if _, err := db.Exec(`UPDATE browser_profiles SET fingerprint_args=?, updated_at=? WHERE profile_id=?`,
			string(fpJSON), time.Now().Format(time.RFC3339), tg.id); err != nil {
			fmt.Printf("            写 fingerprint_args 失败:%v\n", err)
		}
	}

	fmt.Printf("\n需规范化:%d / %d。", changed, len(targets))
	if apply {
		fmt.Println("已写库完成。启动主程序后,UA 与 Client Hints 版本一致;直连实例地理=中国。")
	} else {
		fmt.Println("以上为预览;加 --apply 执行。")
	}
}

func isDirect(proxyID, proxyConfig string) bool {
	pid := strings.TrimSpace(proxyID)
	if pid != "" && pid != "__direct__" {
		return false
	}
	pc := strings.TrimSpace(proxyConfig)
	return pc == "" || strings.EqualFold(pc, "direct://")
}

func modeLabel(apply bool) string {
	if apply {
		return "写库执行"
	}
	return "预览(dry-run)"
}

func argVal(args []string, prefix string) string {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return a[len(prefix):]
		}
	}
	return ""
}

func chromeMajor(ua string) string {
	i := strings.Index(ua, "Chrome/")
	if i < 0 {
		return "?"
	}
	rest := ua[i+len("Chrome/"):]
	if e := strings.IndexByte(rest, '.'); e >= 0 {
		return rest[:e]
	}
	return "?"
}

func diffUA(cur, next []string) string {
	cu, nu := argVal(cur, "--user-agent="), argVal(next, "--user-agent=")
	if cu == "" && nu != "" {
		return fmt.Sprintf("+UA(Chrome/%s)", chromeMajor(nu))
	}
	if cu != nu {
		return fmt.Sprintf("UA %s→%s", chromeMajor(cu), chromeMajor(nu))
	}
	return ""
}

func diffTZ(cur, next []string) string {
	ct, nt := argVal(cur, "--timezone="), argVal(next, "--timezone=")
	if ct != nt {
		return fmt.Sprintf(" tz %s→%s", ct, nt)
	}
	return ""
}

func sameMultiset(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}
