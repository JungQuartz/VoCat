package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"vocat/internal/store"
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

// schedulePreferredPLMN 在 eSIM 切换并恢复射频后，异步应用预设漫游运营商选网，并推送切换结果通知。
func (s *Server) schedulePreferredPLMN(configuredID, physicalID, iccid string) {
	if s.devices == nil {
		return
	}
	plmn := preferredPLMNForICCID(iccid)
	go func() {
		time.Sleep(2500 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var selectErr error
		if plmn != "" {
			cmd := `AT+COPS=1,2,"` + plmn + `"`
			if _, selectErr = s.devices.ExecuteAT(ctx, physicalID, cmd); selectErr != nil {
				s.logger.Warn("auto-select preferred PLMN after eSIM switch failed", "device_id", physicalID, "iccid", iccid, "plmn", plmn, "error", selectErr)
			} else {
				s.logger.Info("auto-selected preferred PLMN after eSIM switch", "device_id", physicalID, "iccid", iccid, "plmn", plmn)
			}
			time.Sleep(3500 * time.Millisecond)
		} else {
			time.Sleep(3 * time.Second)
		}

		// 刷新最新驻网与信号快照用于通知展示
		operator := ""
		regStatus := 0
		signalStr := "--"
		carrierName := ""
		phone := ""
		if snap, err := s.devices.Refresh(ctx, physicalID); err == nil {
			operator = snap.OperatorName
			regStatus = snap.RegistrationStatus
			if snap.RSSIDBm != nil && *snap.RSSIDBm != 0 {
				signalStr = fmt.Sprintf("%d dBm", *snap.RSSIDBm)
			}
			carrierName = snap.SPN
			phone = snap.Phone.Number
		}

		s.notifyESIMSwitchResult(configuredID, iccid, carrierName, phone, plmn, operator, regStatus, signalStr, selectErr)
	}()
}

// notifyESIMSwitchResult 向所有已启用的通知渠道推送 eSIM 切卡与选网最终结果。
func (s *Server) notifyESIMSwitchResult(configuredID, iccid, carrierName, phone, plmn, operator string, regStatus int, signalStr string, selectErr error) {
	ctx, cancel := context.WithTimeout(s.notificationDestinationContext(context.Background()), 15*time.Second)
	defer cancel()

	deviceLabel := configuredID
	if configured, err := s.store.Device(ctx, configuredID); err == nil {
		deviceLabel = firstNonEmpty(configured.Name, configured.ID)
	}
	if strings.TrimSpace(phone) == "" {
		if policy, err := s.store.CardPolicy(ctx, iccid); err == nil {
			phone = strings.TrimSpace(policy.CustomPhoneNumber)
		}
	}

	title := "eSIM 切卡完成"
	statusLine := "未知状态"
	if selectErr != nil {
		title = "eSIM 切卡选网失败"
		statusLine = "失败: " + selectErr.Error()
	} else {
		switch regStatus {
		case 1:
			statusLine = "已驻网 (本地)"
		case 5:
			statusLine = "已驻网 (漫游成功)"
		case 3:
			title = "eSIM 切卡驻网被拒"
			statusLine = "驻网被拒"
		case 2:
			statusLine = "正在搜网中..."
		default:
			statusLine = "未驻网"
		}
	}

	cardDesc := iccid
	if carrierName != "" {
		cardDesc = carrierName + " (" + iccid + ")"
	}
	roamDesc := firstNonEmpty(operator, "搜索中")
	if plmn != "" {
		roamDesc += " [" + plmn + "]"
	}

	lines := []string{
		title,
		"设备  " + deviceLabel,
		"卡片  " + cardDesc,
	}
	if phone != "" {
		lines = append(lines, "号码  "+phone)
	}
	lines = append(lines,
		"漫游  "+roamDesc,
		"信号  "+signalStr,
		"状态  "+statusLine,
		"时间  "+time.Now().Local().Format("2006-01-02 15:04:05"),
	)
	text := strings.Join(lines, "\n")

	for _, channel := range notificationChannels {
		setting, err := s.store.NotificationSetting(ctx, channel)
		if errors.Is(err, store.ErrNotFound) || (err == nil && !setting.Enabled) {
			continue
		}
		if err != nil {
			continue
		}
		var config map[string]any
		if err := json.Unmarshal(setting.Config, &config); err != nil {
			continue
		}
		var sendErr error
		switch channel {
		case "telegram":
			sendErr = sendTelegramTextNotification(ctx, config, configuredID, text)
		case "bark":
			sendErr = sendBarkTextNotification(ctx, config, configuredID, title, text)
		case "email":
			sendErr = sendEmailTextNotification(ctx, config, configuredID, title, text)
		case "pushplus":
			sendErr = sendPushplusTextNotification(ctx, config, configuredID, title, text)
		case "meow":
			sendErr = sendMeowNotification(ctx, config, &configuredID, title, text)
		case "wecom":
			sendErr = sendWecomNotification(ctx, config, &configuredID, map[string]string{"title": title, "content": text})
		case "lark":
			sendErr = sendLarkNotification(ctx, config, &configuredID, map[string]string{"title": title, "content": text})
		}
		if sendErr != nil {
			s.logger.Warn("send eSIM switch notification failed", "channel", channel, "device_id", configuredID, "error", sendErr)
		}
	}
}
