package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
)

// configField is one editable setting the console renders. Everything that
// changes a listen port, a path or a credential is deliberately absent: those are
// edited in the file by hand, where the consequences are obvious.
type configField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Value any    `json:"value"`
	Min   *int   `json:"min,omitempty"`
	Max   *int   `json:"max,omitempty"`
	Hint  string `json:"hint,omitempty"`
}

func bound(min, max int) (*int, *int) { return &min, &max }

// configFields renders the editable set from a configuration.
func configFields(cfg config.Config) []configField {
	portMin, portMax := bound(1, 65535)
	connMin, connMax := bound(1, 65535)
	idleMin, idleMax := bound(0, 3600)
	turnMin, turnMax := bound(500, 60000)
	regMin, regMax := bound(0, 100000)
	return []configField{
		{Key: "network.listenPort", Label: "游戏/大厅端口", Type: "int", Value: cfg.Network.ListenPort, Min: portMin, Max: portMax},
		{Key: "network.maxConnections", Label: "游戏连接上限", Type: "int", Value: cfg.Network.MaxConnections, Min: connMin, Max: connMax},
		{Key: "network.idleTimeoutSeconds", Label: "游戏空闲超时（秒，0=关闭）", Type: "int", Value: cfg.Network.IdleTimeoutSeconds, Min: idleMin, Max: idleMax},
		{Key: "relay.enabled", Label: "启用中继", Type: "bool", Value: cfg.Relay.Enabled},
		{Key: "relay.listenPort", Label: "中继端口", Type: "int", Value: cfg.Relay.ListenPort, Min: portMin, Max: portMax},
		{Key: "relay.advertisedAddress", Label: "中继对外 IP（下发客户端）", Type: "string", Value: cfg.Relay.AdvertisedAddress, Hint: "对外开服必须填公网 IP"},
		{Key: "relay.maxConnections", Label: "中继连接上限", Type: "int", Value: cfg.Relay.MaxConnections, Min: connMin, Max: connMax},
		{Key: "relay.idleTimeoutSeconds", Label: "中继空闲超时（秒，0=关闭）", Type: "int", Value: cfg.Relay.IdleTimeoutSeconds, Min: idleMin, Max: idleMax},
		{Key: "zombie.humanTurnMs", Label: "僵尸：人类变身等待（毫秒）", Type: "int", Value: cfg.Zombie.HumanTurnMs, Min: turnMin, Max: turnMax},
		{Key: "zombie.zombiniTurnMs", Label: "僵尸：小僵尸复活等待（毫秒）", Type: "int", Value: cfg.Zombie.ZombiniTurnMs, Min: turnMin, Max: turnMax},
		{Key: "logging.level", Label: "日志级别", Type: "string", Value: cfg.Logging.Level, Hint: "trace / debug / info / warn / error"},
		{Key: "authentication.inviteCode", Label: "注册邀请码（空=开放注册）", Type: "string", Value: cfg.Authentication.InviteCode},
		{Key: "authentication.registrationsPerHour", Label: "每 IP 每小时注册上限", Type: "int", Value: cfg.Authentication.RegistrationsPerHour, Min: regMin, Max: regMax},
		{Key: "authentication.registrationsPerDay", Label: "每 IP 每天注册上限", Type: "int", Value: cfg.Authentication.RegistrationsPerDay, Min: regMin, Max: regMax},
	}
}

func (s *Server) getConfig(w http.ResponseWriter, _ *http.Request) {
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"path":   s.cfgPath,
		"fields": configFields(cfg),
		"note":   "保存后需要手动重启服务端才会生效；数据库、证书、管理后台与文件路径不在此编辑。",
	})
}

// saveConfig merges the posted fields into the file, validates the result and
// only then replaces it. A rejected value never touches the real file.
func (s *Server) saveConfig(w http.ResponseWriter, r *http.Request) {
	var patch map[string]json.RawMessage
	if !decode(w, r, &patch) {
		return
	}
	if len(patch) == 0 {
		writeError(w, http.StatusBadRequest, "没有要保存的字段")
		return
	}

	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	raw, err := os.ReadFile(s.cfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置文件失败："+err.Error())
		return
	}
	var cfg config.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "现有配置文件不是合法 JSON："+err.Error())
		return
	}
	for key, value := range patch {
		if err := applyConfigField(&cfg, key, value); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("%s：%v", key, err))
			return
		}
	}

	encoded, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	encoded = append(encoded, '\n')

	// Validate exactly the bytes that would be written: a hand-edited file can be
	// broken in ways the merged struct alone would not show.
	temp := s.cfgPath + ".new"
	if err := os.WriteFile(temp, encoded, fileModeOf(s.cfgPath)); err != nil {
		writeError(w, http.StatusInternalServerError, "写入临时文件失败："+err.Error())
		return
	}
	if _, err := config.Load(temp); err != nil {
		_ = os.Remove(temp)
		writeError(w, http.StatusBadRequest, "校验失败，未保存："+err.Error())
		return
	}
	if err := os.WriteFile(s.cfgPath+".bak", raw, fileModeOf(s.cfgPath)); err != nil {
		_ = os.Remove(temp)
		writeError(w, http.StatusInternalServerError, "备份旧配置失败："+err.Error())
		return
	}
	if err := os.Rename(temp, s.cfgPath); err != nil {
		_ = os.Remove(temp)
		writeError(w, http.StatusInternalServerError, "替换配置文件失败："+err.Error())
		return
	}
	s.log.Info("[Admin] configuration written", "path", s.cfgPath, "fields", len(patch))
	writeJSON(w, map[string]any{"ok": true, "restartRequired": true})
}

// fileModeOf mirrors the current file's permissions so a protected config stays
// protected after a rewrite.
func fileModeOf(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o600
}

// applyConfigField sets one whitelisted key. Anything else is refused, which is
// what keeps paths, credentials and the console's own settings out of reach.
func applyConfigField(cfg *config.Config, key string, raw json.RawMessage) error {
	switch key {
	case "network.listenPort":
		return assign(raw, &cfg.Network.ListenPort)
	case "network.maxConnections":
		return assign(raw, &cfg.Network.MaxConnections)
	case "network.idleTimeoutSeconds":
		return assign(raw, &cfg.Network.IdleTimeoutSeconds)
	case "relay.enabled":
		return assign(raw, &cfg.Relay.Enabled)
	case "relay.listenPort":
		return assign(raw, &cfg.Relay.ListenPort)
	case "relay.advertisedAddress":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return errors.New("需要字符串")
		}
		if value != "" && net.ParseIP(value) == nil {
			return errors.New("需要 IPv4 地址或留空")
		}
		cfg.Relay.AdvertisedAddress = value
		return nil
	case "relay.maxConnections":
		return assign(raw, &cfg.Relay.MaxConnections)
	case "relay.idleTimeoutSeconds":
		return assign(raw, &cfg.Relay.IdleTimeoutSeconds)
	case "zombie.humanTurnMs":
		return assign(raw, &cfg.Zombie.HumanTurnMs)
	case "zombie.zombiniTurnMs":
		return assign(raw, &cfg.Zombie.ZombiniTurnMs)
	case "logging.level":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return errors.New("需要字符串")
		}
		cfg.Logging.Level = value
		return nil
	case "authentication.inviteCode":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return errors.New("需要字符串")
		}
		cfg.Authentication.InviteCode = value
		return nil
	case "authentication.registrationsPerHour":
		return assign(raw, &cfg.Authentication.RegistrationsPerHour)
	case "authentication.registrationsPerDay":
		return assign(raw, &cfg.Authentication.RegistrationsPerDay)
	default:
		return fmt.Errorf("不可在线编辑的字段")
	}
}

// assign decodes one JSON scalar into the field it belongs to.
func assign[T any](raw json.RawMessage, target *T) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New("类型不匹配")
	}
	return nil
}

// configPath returns the absolute path the console shows and edits.
func (s *Server) configPath() string {
	if absolute, err := filepath.Abs(s.cfgPath); err == nil {
		return absolute
	}
	return s.cfgPath
}
