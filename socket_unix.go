//go:build unix

package water

func platformSpecificFd(fd uintptr) int {
	return int(fd)
}
