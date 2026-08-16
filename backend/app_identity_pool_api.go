package backend

import (
	"fmt"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/identity"
)

// IdentityPoolRecord 身份池模板记录(Wails 绑定用别名 → models.ts)。
type IdentityPoolRecord = identity.PoolRecord

// IdentityPoolProblem 一条不自洽的身份池记录及其问题。
type IdentityPoolProblem struct {
	ID       string           `json:"id"`
	UAFull   string           `json:"uaFull"`
	Platform string           `json:"platform"`
	Issues   []identity.Issue `json:"issues"`
}

// IdentityPoolValidateAllReport 全量自洽检查汇总。
type IdentityPoolValidateAllReport struct {
	Total    int                   `json:"total"`
	OKCount  int                   `json:"okCount"`
	BadCount int                   `json:"badCount"`
	Problems []IdentityPoolProblem `json:"problems"`
}

func (a *App) identityService() (*browser.IdentityService, error) {
	if a.browserMgr == nil || a.browserMgr.IdentityService == nil {
		return nil, fmt.Errorf("身份池服务不可用")
	}
	return a.browserMgr.IdentityService, nil
}

// IdentityPoolList 返回身份池全部模板记录。
func (a *App) IdentityPoolList() []IdentityPoolRecord {
	svc, err := a.identityService()
	if err != nil {
		return []IdentityPoolRecord{}
	}
	return svc.PoolRecords()
}

// IdentityPoolCreate 新增一条身份池模板(自动赋 ID)。
func (a *App) IdentityPoolCreate(rec IdentityPoolRecord) (IdentityPoolRecord, error) {
	svc, err := a.identityService()
	if err != nil {
		return IdentityPoolRecord{}, err
	}
	return svc.AddPoolRecord(rec)
}

// IdentityPoolUpdate 按 ID 更新一条身份池模板。
func (a *App) IdentityPoolUpdate(id string, rec IdentityPoolRecord) (IdentityPoolRecord, error) {
	svc, err := a.identityService()
	if err != nil {
		return IdentityPoolRecord{}, err
	}
	return svc.UpdatePoolRecord(id, rec)
}

// IdentityPoolDelete 按 ID 删除一条身份池模板。
func (a *App) IdentityPoolDelete(id string) error {
	svc, err := a.identityService()
	if err != nil {
		return err
	}
	return svc.DeletePoolRecord(id)
}

// IdentityPoolValidate 校验一条身份池模板的自洽性。
func (a *App) IdentityPoolValidate(rec IdentityPoolRecord) FingerprintValidationResult {
	svc, err := a.identityService()
	if err != nil {
		return FingerprintValidationResult{OK: false, Issues: []identity.Issue{{Field: "service", Message: err.Error(), Severity: identity.SeverityError}}}
	}
	return svc.ValidatePoolRecord(rec)
}

// IdentityPoolValidateAll 对全部记录做自洽检查,汇总不通过的条目。
func (a *App) IdentityPoolValidateAll() IdentityPoolValidateAllReport {
	report := IdentityPoolValidateAllReport{Problems: []IdentityPoolProblem{}}
	svc, err := a.identityService()
	if err != nil {
		return report
	}
	for _, rec := range svc.PoolRecords() {
		report.Total++
		res := svc.ValidatePoolRecord(rec)
		if res.OK {
			report.OKCount++
			continue
		}
		report.BadCount++
		report.Problems = append(report.Problems, IdentityPoolProblem{
			ID:       rec.ID,
			UAFull:   rec.UAFull,
			Platform: rec.Platform,
			Issues:   res.Issues,
		})
	}
	return report
}

// IdentityPoolRestoreDefaults 把身份池恢复为内嵌默认。
func (a *App) IdentityPoolRestoreDefaults() error {
	svc, err := a.identityService()
	if err != nil {
		return err
	}
	return svc.RestorePoolDefaults()
}
