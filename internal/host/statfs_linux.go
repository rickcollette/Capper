package host

import "syscall"

type syscallStatfs struct{ s syscall.Statfs_t }

func statfs(path string, out *syscallStatfs) error {
	return syscall.Statfs(path, &out.s)
}

func (s *syscallStatfs) available() int64 {
	return int64(s.s.Bavail) * int64(s.s.Bsize)
}
