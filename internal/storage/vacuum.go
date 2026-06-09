package storage

// Vacuum 回收 SQLite 文件空洞（VACUUM 重建库文件）。
//
// 删行只把页标记为空闲、不缩文件（R99）；长跑下剪枝后磁盘占用只增不减。
// VACUUM 重写整库回收空间，但开销大（全库重写 + 需约等于库大小的临时空间），
// 故按月 / 手动触发，绝不每日跑。调用方负责频率门控（见 main.runMaintenance）。
func Vacuum() error {
	_, err := db.Exec(`VACUUM`)
	return err
}
