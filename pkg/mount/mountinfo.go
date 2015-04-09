package mount

// 这个数据结构完全的映射了/proc/self/mountinfo文件中每一行的内容
type MountInfo struct {
	Id, Parent, Major, Minor         int
	Root, Mountpoint, Opts, Optional string
	Fstype, Source, VfsOpts          string
}
