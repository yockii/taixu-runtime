package storage

// Contact 生命体对话过的对象（A 社交联动 / B 主动发消息）。
//
// 前瞻（Phase 4）：peer 将不止"用户"，还含其他生命体 / 世界服务；channel 多渠道化；
// 接 reputation / social 资源与 Relationship/Pact。见 migrations/005_contacts.sql 注释。
type Contact struct {
	ID       int64  `json:"id"`
	Channel  string `json:"channel"`
	PeerID   string `json:"peer_id"`
	PeerName string `json:"peer_name,omitempty"`
	ChatType string `json:"chat_type"` // "direct"（单聊）/ "group"（群聊）
	MsgCount int64  `json:"msg_count"`
	FirstAt  int64  `json:"first_at"`
	LastAt   int64  `json:"last_at"`
}

// ChatTypeDirect / ChatTypeGroup 会话类型枚举。空串归一为 direct。
const (
	ChatTypeDirect = "direct"
	ChatTypeGroup  = "group"
)

// NormChatType 归一会话类型；未知 / 空 → direct。
func NormChatType(t string) string {
	if t == ChatTypeGroup {
		return ChatTypeGroup
	}
	return ChatTypeDirect
}

// peerKey 归一空 peer（cli 注入常无 from）为 'local'。
func peerKey(peer string) string {
	if peer == "" {
		return "local"
	}
	return peer
}

// UpsertContact 记录/更新一次交互：msg_count++ + last_at 推进；首见则插入。
// chatType 区分单聊 / 群聊（群聊语义下 peer 是群 id）。
func UpsertContact(lifeID, channel, peer, peerName, chatType string, ts int64) error {
	peer = peerKey(peer)
	chatType = NormChatType(chatType)
	res, err := db.Exec(`
		UPDATE contact SET msg_count = msg_count + 1, last_at = ?,
		    peer_name = COALESCE(NULLIF(?, ''), peer_name), chat_type = ?
		WHERE life_id = ? AND channel = ? AND peer_id = ?`,
		ts, peerName, chatType, lifeID, channel, peer)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err = db.Exec(`
		INSERT INTO contact (life_id, channel, peer_id, peer_name, chat_type, msg_count, first_at, last_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		lifeID, channel, peer, nullStr(peerName), chatType, ts, ts)
	return err
}

// MostRecentContact 取最近交互的联系人（主动发消息选目标）。无则 (nil, nil)。
func MostRecentContact(lifeID string) (*Contact, error) {
	var c Contact
	err := db.QueryRow(`
		SELECT id, channel, peer_id, COALESCE(peer_name,''), chat_type, msg_count, first_at, last_at
		FROM contact WHERE life_id = ?
		ORDER BY last_at DESC LIMIT 1`, lifeID).
		Scan(&c.ID, &c.Channel, &c.PeerID, &c.PeerName, &c.ChatType, &c.MsgCount, &c.FirstAt, &c.LastAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// GetContact 按 (channel, peer) 取单个联系人；无则 (nil, nil)。供 reflex 自我标识"在和谁会话"。
func GetContact(lifeID, channel, peer string) (*Contact, error) {
	peer = peerKey(peer)
	var c Contact
	err := db.QueryRow(`
		SELECT id, channel, peer_id, COALESCE(peer_name,''), chat_type, msg_count, first_at, last_at
		FROM contact WHERE life_id = ? AND channel = ? AND peer_id = ?`,
		lifeID, channel, peer).
		Scan(&c.ID, &c.Channel, &c.PeerID, &c.PeerName, &c.ChatType, &c.MsgCount, &c.FirstAt, &c.LastAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// PeerKey 归一空 peer（导出，供同包外的会话作用域键拼接复用同一归一规则）。
func PeerKey(peer string) string { return peerKey(peer) }

// ListContacts 全部联系人（观察用）。
func ListContacts(lifeID string, limit int) ([]Contact, error) {
	rows, err := db.Query(`
		SELECT id, channel, peer_id, COALESCE(peer_name,''), chat_type, msg_count, first_at, last_at
		FROM contact WHERE life_id = ? ORDER BY last_at DESC LIMIT ?`, lifeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Contact{}
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Channel, &c.PeerID, &c.PeerName, &c.ChatType, &c.MsgCount, &c.FirstAt, &c.LastAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
