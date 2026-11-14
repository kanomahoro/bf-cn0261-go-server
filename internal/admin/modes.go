package admin

import "strconv"

// modeNames is the console's mode table. It has to list every mode this server
// runs, because the results page, the room list and the CSV export all name a
// mode through it; an unknown number shows as "模式 N" and hides a whole mode from
// the operator. Keep it in step with internal/admin/index.html's MODE map and the
// results filter (a test pins both).
//
// 5 and 6 (夺星) are named only so an old database row still reads sensibly: this
// client has no such mode to select.
var modeNames = map[uint32]string{
	1:   "竞技",
	3:   "道具",
	5:   "夺星",
	6:   "夺星",
	8:   "僵尸",
	12:  "狙击",
	13:  "热血个人",
	14:  "热血团队",
	16:  "僵尸BOSS",
	18:  "热血狙击",
	19:  "爆破",
	20:  "队长战",
	101: "海盗",
}

// modeName names one mode for the console and the CSV export.
func modeName(mode uint32) string {
	if name, ok := modeNames[mode]; ok {
		return name
	}
	return "模式 " + strconv.FormatUint(uint64(mode), 10)
}
