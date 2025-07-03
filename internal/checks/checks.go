package checks

import (
    "github.com/leozw/uptime-guardian/internal/db"
)

type Runner interface {
    Check(monitor *db.Monitor, region string) *db.CheckResult
}