package storage

import "database/sql"

// SkillInstance SKILL.md 种子在本生命体内的有状态实例（docs/SKILLS-AND-TOOLS §2）。
type SkillInstance struct {
	ID           string  `json:"id"`            // seed_hash
	LifeID       string  `json:"life_id"`
	Name         string  `json:"name"`
	SeedRef      string  `json:"seed_ref"`      // SKILL.md sha256
	SeedVersion  string  `json:"seed_version"`
	Description  string  `json:"description"`
	Lanes        string  `json:"lanes"`         // JSON 数组串
	AllowedTools string  `json:"allowed_tools"` // JSON 数组串
	Status       string  `json:"status"`
	PendingDeps  string  `json:"pending_deps,omitempty"` // JSON
	Mastery      float64 `json:"mastery"`
	UsedCount    int64   `json:"used_count"`
	LastUsedAt   int64   `json:"last_used_at,omitempty"`
	InstallPath  string  `json:"install_path,omitempty"`
	AuthoredFrom string  `json:"authored_from,omitempty"` // "" 外部投放 / "interest_seed#N" 自创
	CreatedAt    int64   `json:"created_at"`
}

// SkillDependency 依赖装载审计行（append-only）。
type SkillDependency struct {
	ID          int64  `json:"id"`
	SkillID     string `json:"skill_id"`
	Runtime     string `json:"runtime"` // python / node
	Package     string `json:"package"`
	Version     string `json:"version"`
	InstallHash string `json:"install_hash,omitempty"`
	InstalledBy string `json:"installed_by"` // user_approve / auto_approve / bundle
	InstalledAt int64  `json:"installed_at"`
}

// UpsertSkillInstance 插入或替换一个 skill 实例（按 id 主键）。
func UpsertSkillInstance(s *SkillInstance) error {
	_, err := db.Exec(`
		INSERT INTO skill_instance
			(id, life_id, name, seed_ref, seed_version, description, lanes, allowed_tools,
			 status, pending_deps, mastery, used_count, last_used_at, install_path, authored_from, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, seed_ref=excluded.seed_ref, seed_version=excluded.seed_version,
			description=excluded.description, lanes=excluded.lanes, allowed_tools=excluded.allowed_tools,
			status=excluded.status, pending_deps=excluded.pending_deps, install_path=excluded.install_path,
			authored_from=COALESCE(NULLIF(excluded.authored_from,''), skill_instance.authored_from)`,
		s.ID, s.LifeID, s.Name, s.SeedRef, nullStr(s.SeedVersion), nullStr(s.Description),
		nullStr(s.Lanes), nullStr(s.AllowedTools), s.Status, nullStr(s.PendingDeps),
		s.Mastery, s.UsedCount, nullInt(s.LastUsedAt), nullStr(s.InstallPath), nullStr(s.AuthoredFrom), s.CreatedAt)
	return err
}

// GetSkillInstance 按 id 取。未找到返 (nil, nil)。
func GetSkillInstance(id string) (*SkillInstance, error) {
	return scanSkill(db.QueryRow(`
		SELECT id, life_id, name, seed_ref, COALESCE(seed_version,''), COALESCE(description,''),
		       COALESCE(lanes,''), COALESCE(allowed_tools,''), status, COALESCE(pending_deps,''),
		       mastery, used_count, COALESCE(last_used_at,0), COALESCE(install_path,''),
		       COALESCE(authored_from,''), created_at
		FROM skill_instance WHERE id = ?`, id))
}

// ListSkillInstances 列本生命体所有 skill（按 created_at desc）。
func ListSkillInstances(lifeID string, limit int) ([]SkillInstance, error) {
	rows, err := db.Query(`
		SELECT id, life_id, name, seed_ref, COALESCE(seed_version,''), COALESCE(description,''),
		       COALESCE(lanes,''), COALESCE(allowed_tools,''), status, COALESCE(pending_deps,''),
		       mastery, used_count, COALESCE(last_used_at,0), COALESCE(install_path,''),
		       COALESCE(authored_from,''), created_at
		FROM skill_instance WHERE life_id = ?
		ORDER BY created_at DESC LIMIT ?`, lifeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SkillInstance{}
	for rows.Next() {
		s, err := scanSkillRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// UpdateSkillStatus 改状态（可选清 pending_deps）。
func UpdateSkillStatus(id, status string, clearPending bool) error {
	if clearPending {
		_, err := db.Exec(`UPDATE skill_instance SET status=?, pending_deps=NULL WHERE id=?`, status, id)
		return err
	}
	_, err := db.Exec(`UPDATE skill_instance SET status=? WHERE id=?`, status, id)
	return err
}

// SetSkillReady 置 ready + install_path + 清 pending。
func SetSkillReady(id, installPath string) error {
	_, err := db.Exec(`UPDATE skill_instance SET status='ready', install_path=?, pending_deps=NULL WHERE id=?`,
		installPath, id)
	return err
}

// SetSkillAuthoredFrom 标记 skill 血缘（自创来源）。
func SetSkillAuthoredFrom(id, authoredFrom string) error {
	_, err := db.Exec(`UPDATE skill_instance SET authored_from = ? WHERE id = ?`, authoredFrom, id)
	return err
}

// BumpSkillUsed used_count++ + last_used_at。
func BumpSkillUsed(id string, ts int64) error {
	_, err := db.Exec(`UPDATE skill_instance SET used_count=used_count+1, last_used_at=? WHERE id=?`, ts, id)
	return err
}

// InsertSkillDependency 记一条依赖装载审计。
func InsertSkillDependency(d *SkillDependency) error {
	_, err := db.Exec(`
		INSERT INTO skill_dependency (skill_id, runtime, package, version, install_hash, installed_by, installed_at)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(skill_id, runtime, package) DO UPDATE SET
			version=excluded.version, install_hash=excluded.install_hash,
			installed_by=excluded.installed_by, installed_at=excluded.installed_at`,
		d.SkillID, d.Runtime, d.Package, d.Version, nullStr(d.InstallHash), d.InstalledBy, d.InstalledAt)
	return err
}

// ListSkillDependencies 列某 skill 的依赖。
func ListSkillDependencies(skillID string) ([]SkillDependency, error) {
	rows, err := db.Query(`
		SELECT id, skill_id, runtime, package, version, COALESCE(install_hash,''), COALESCE(installed_by,''), installed_at
		FROM skill_dependency WHERE skill_id = ? ORDER BY id`, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SkillDependency{}
	for rows.Next() {
		var d SkillDependency
		if err := rows.Scan(&d.ID, &d.SkillID, &d.Runtime, &d.Package, &d.Version,
			&d.InstallHash, &d.InstalledBy, &d.InstalledAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func scanSkill(row *sql.Row) (*SkillInstance, error) {
	var s SkillInstance
	err := row.Scan(&s.ID, &s.LifeID, &s.Name, &s.SeedRef, &s.SeedVersion, &s.Description,
		&s.Lanes, &s.AllowedTools, &s.Status, &s.PendingDeps,
		&s.Mastery, &s.UsedCount, &s.LastUsedAt, &s.InstallPath, &s.AuthoredFrom, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanSkillRows(rows *sql.Rows) (*SkillInstance, error) {
	var s SkillInstance
	err := rows.Scan(&s.ID, &s.LifeID, &s.Name, &s.SeedRef, &s.SeedVersion, &s.Description,
		&s.Lanes, &s.AllowedTools, &s.Status, &s.PendingDeps,
		&s.Mastery, &s.UsedCount, &s.LastUsedAt, &s.InstallPath, &s.AuthoredFrom, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
