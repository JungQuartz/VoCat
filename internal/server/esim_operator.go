package server

import (
	"context"
	"strings"
	"time"
)

// preferredPLMNForICCID 返回特定 eSIM 卡号在当前环境下的首选漫游运营商 PLMN 码。
func preferredPLMNForICCID(iccid string) string {
	switch strings.TrimSpace(iccid) {
	case "8985200014632405653": // 香港 Club SIM eSIM -> 中国电信
		return "46011"
	case "89852350826081220610": // Saily WEBBING eSIM -> 中国移动
		return "46000"
	default:
		return ""
	}
}

// schedulePreferredPLMN 在 eSIM 切换并恢复射频后，异步应用预设的漫游运营商选网。
func (s *Server) schedulePreferredPLMN(physicalID, iccid string) {
	plmn := preferredPLMNForICCID(iccid)
	if plmn == "" || s.devices == nil {
		return
	}
	go func() {
		time.Sleep(2500 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := `AT+COPS=1,2,"` + plmn + `"`
		if _, err := s.devices.ExecuteAT(ctx, physicalID, cmd); err != nil {
			s.logger.Warn("auto-select preferred PLMN after eSIM switch failed", "device_id", physicalID, "iccid", iccid, "plmn", plmn, "error", err)
			return
		}
		s.logger.Info("auto-selected preferred PLMN after eSIM switch", "device_id", physicalID, "iccid", iccid, "plmn", plmn)
	}()
}
