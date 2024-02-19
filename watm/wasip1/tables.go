package wasip1

import (
	"syscall"
)

// errno is just a copy of syscall.Errno from the Go standard library.
//
// WATER error code is defined as the negative value of errno.
const (
	E2BIG           errno = 1
	EACCES          errno = 2
	EADDRINUSE      errno = 3
	EADDRNOTAVAIL   errno = 4
	EAFNOSUPPORT    errno = 5
	EAGAIN          errno = 6
	EALREADY        errno = 7
	EBADF           errno = 8
	EBADMSG         errno = 9
	EBUSY           errno = 10
	ECANCELED       errno = 11
	ECHILD          errno = 12
	ECONNABORTED    errno = 13
	ECONNREFUSED    errno = 14
	ECONNRESET      errno = 15
	EDEADLK         errno = 16
	EDESTADDRREQ    errno = 17
	EDOM            errno = 18
	EDQUOT          errno = 19
	EEXIST          errno = 20
	EFAULT          errno = 21
	EFBIG           errno = 22
	EHOSTUNREACH    errno = 23
	EIDRM           errno = 24
	EILSEQ          errno = 25
	EINPROGRESS     errno = 26
	EINTR           errno = 27
	EINVAL          errno = 28
	EIO             errno = 29
	EISCONN         errno = 30
	EISDIR          errno = 31
	ELOOP           errno = 32
	EMFILE          errno = 33
	EMLINK          errno = 34
	EMSGSIZE        errno = 35
	EMULTIHOP       errno = 36
	ENAMETOOLONG    errno = 37
	ENETDOWN        errno = 38
	ENETRESET       errno = 39
	ENETUNREACH     errno = 40
	ENFILE          errno = 41
	ENOBUFS         errno = 42
	ENODEV          errno = 43
	ENOENT          errno = 44
	ENOEXEC         errno = 45
	ENOLCK          errno = 46
	ENOLINK         errno = 47
	ENOMEM          errno = 48
	ENOMSG          errno = 49
	ENOPROTOOPT     errno = 50
	ENOSPC          errno = 51
	ENOSYS          errno = 52
	ENOTCONN        errno = 53
	ENOTDIR         errno = 54
	ENOTEMPTY       errno = 55
	ENOTRECOVERABLE errno = 56
	ENOTSOCK        errno = 57
	ENOTSUP         errno = 58
	ENOTTY          errno = 59
	ENXIO           errno = 60
	EOVERFLOW       errno = 61
	EOWNERDEAD      errno = 62
	EPERM           errno = 63
	EPIPE           errno = 64
	EPROTO          errno = 65
	EPROTONOSUPPORT errno = 66
	EPROTOTYPE      errno = 67
	ERANGE          errno = 68
	EROFS           errno = 69
	ESPIPE          errno = 70
	ESRCH           errno = 71
	ESTALE          errno = 72
	ETIMEDOUT       errno = 73
	ETXTBSY         errno = 74
	EXDEV           errno = 75
	ENOTCAPABLE     errno = 76
	// needed by src/net/error_unix_test.go
	EOPNOTSUPP = ENOTSUP
)

// TODO: Auto-generate some day. (Hard-coded in binaries so not likely to change.)
var errorstr = [...]string{
	E2BIG:           "Argument list too long",
	EACCES:          "Permission denied",
	EADDRINUSE:      "Address already in use",
	EADDRNOTAVAIL:   "Address not available",
	EAFNOSUPPORT:    "Address family not supported by protocol family",
	EAGAIN:          "Try again",
	EALREADY:        "Socket already connected",
	EBADF:           "Bad file number",
	EBADMSG:         "Trying to read unreadable message",
	EBUSY:           "Device or resource busy",
	ECANCELED:       "Operation canceled.",
	ECHILD:          "No child processes",
	ECONNABORTED:    "Connection aborted",
	ECONNREFUSED:    "Connection refused",
	ECONNRESET:      "Connection reset by peer",
	EDEADLK:         "Deadlock condition",
	EDESTADDRREQ:    "Destination address required",
	EDOM:            "Math arg out of domain of func",
	EDQUOT:          "Quota exceeded",
	EEXIST:          "File exists",
	EFAULT:          "Bad address",
	EFBIG:           "File too large",
	EHOSTUNREACH:    "Host is unreachable",
	EIDRM:           "Identifier removed",
	EILSEQ:          "EILSEQ",
	EINPROGRESS:     "Connection already in progress",
	EINTR:           "Interrupted system call",
	EINVAL:          "Invalid argument",
	EIO:             "I/O error",
	EISCONN:         "Socket is already connected",
	EISDIR:          "Is a directory",
	ELOOP:           "Too many symbolic links",
	EMFILE:          "Too many open files",
	EMLINK:          "Too many links",
	EMSGSIZE:        "Message too long",
	EMULTIHOP:       "Multihop attempted",
	ENAMETOOLONG:    "File name too long",
	ENETDOWN:        "Network interface is not configured",
	ENETRESET:       "Network dropped connection on reset",
	ENETUNREACH:     "Network is unreachable",
	ENFILE:          "File table overflow",
	ENOBUFS:         "No buffer space available",
	ENODEV:          "No such device",
	ENOENT:          "No such file or directory",
	ENOEXEC:         "Exec format error",
	ENOLCK:          "No record locks available",
	ENOLINK:         "The link has been severed",
	ENOMEM:          "Out of memory",
	ENOMSG:          "No message of desired type",
	ENOPROTOOPT:     "Protocol not available",
	ENOSPC:          "No space left on device",
	ENOSYS:          "Not implemented on wasip1",
	ENOTCONN:        "Socket is not connected",
	ENOTDIR:         "Not a directory",
	ENOTEMPTY:       "Directory not empty",
	ENOTRECOVERABLE: "State not recoverable",
	ENOTSOCK:        "Socket operation on non-socket",
	ENOTSUP:         "Not supported",
	ENOTTY:          "Not a typewriter",
	ENXIO:           "No such device or address",
	EOVERFLOW:       "Value too large for defined data type",
	EOWNERDEAD:      "Owner died",
	EPERM:           "Operation not permitted",
	EPIPE:           "Broken pipe",
	EPROTO:          "Protocol error",
	EPROTONOSUPPORT: "Unknown protocol",
	EPROTOTYPE:      "Protocol wrong type for socket",
	ERANGE:          "Math result not representable",
	EROFS:           "Read-only file system",
	ESPIPE:          "Illegal seek",
	ESRCH:           "No such process",
	ESTALE:          "Stale file handle",
	ETIMEDOUT:       "Connection timed out",
	ETXTBSY:         "Text file busy",
	EXDEV:           "Cross-device link",
	ENOTCAPABLE:     "Capabilities insufficient",
}

var mapSyscall2Errno = map[syscall.Errno]errno{
	syscall.E2BIG:         E2BIG,
	syscall.EACCES:        EACCES,
	syscall.EADDRINUSE:    EADDRINUSE,
	syscall.EADDRNOTAVAIL: EADDRNOTAVAIL,
	syscall.EAFNOSUPPORT:  EAFNOSUPPORT,
	syscall.EAGAIN:        EAGAIN,
	syscall.EALREADY:      EALREADY,
	syscall.EBADF:         EBADF,
	syscall.EBADMSG:       EBADMSG,
	syscall.EBUSY:         EBUSY,
	syscall.ECANCELED:     ECANCELED,
	syscall.ECHILD:        ECHILD,
	syscall.ECONNABORTED:  ECONNABORTED,
	syscall.ECONNREFUSED:  ECONNREFUSED,
	syscall.ECONNRESET:    ECONNRESET,
	syscall.EDEADLK:       EDEADLK,
	syscall.EDESTADDRREQ:  EDESTADDRREQ,
	syscall.EDOM:          EDOM,
	syscall.EDQUOT:        EDQUOT,
	syscall.EEXIST:        EEXIST,
	syscall.EFAULT:        EFAULT,
	syscall.EFBIG:         EFBIG,
	syscall.EHOSTUNREACH:  EHOSTUNREACH,
	syscall.EIDRM:         EIDRM,
	syscall.EILSEQ:        EILSEQ,
	syscall.EINPROGRESS:   EINPROGRESS,
	syscall.EINTR:         EINTR,
	syscall.EINVAL:        EINVAL,
	syscall.EIO:           EIO,
	syscall.EISCONN:       EISCONN,
	syscall.EISDIR:        EISDIR,
	syscall.ELOOP:         ELOOP,
	syscall.EMFILE:        EMFILE,
	syscall.EMLINK:        EMLINK,
	syscall.EMSGSIZE:      EMSGSIZE,
	syscall.EMULTIHOP:     EMULTIHOP,
	syscall.ENAMETOOLONG:  ENAMETOOLONG,
	syscall.ENETDOWN:      ENETDOWN,
	syscall.ENETRESET:     ENETRESET,
	syscall.ENETUNREACH:   ENETUNREACH,
	syscall.ENFILE:        ENFILE,
	syscall.ENOBUFS:       ENOBUFS,
	syscall.ENODEV:        ENODEV,
	syscall.ENOENT:        ENOENT,
	syscall.ENOEXEC:       ENOEXEC,
	syscall.ENOLCK:        ENOLCK,
	syscall.ENOLINK:       ENOLINK,
	syscall.ENOMEM:        ENOMEM,
	syscall.ENOMSG:        ENOMSG,
	syscall.ENOPROTOOPT:   ENOPROTOOPT,
	syscall.ENOSPC:        ENOSPC,
	syscall.ENOSYS:        ENOSYS,
	syscall.ENOTCONN:      ENOTCONN,
	syscall.ENOTDIR:       ENOTDIR,
	syscall.ENOTEMPTY:     ENOTEMPTY,
	// syscall.ENOTRECOVERABLE: ENOTRECOVERABLE,
	syscall.ENOTSOCK:  ENOTSOCK,
	syscall.ENOTSUP:   ENOTSUP,
	syscall.ENOTTY:    ENOTTY,
	syscall.ENXIO:     ENXIO,
	syscall.EOVERFLOW: EOVERFLOW,
	// syscall.EOWNERDEAD:      EOWNERDEAD,
	syscall.EPERM:           EPERM,
	syscall.EPIPE:           EPIPE,
	syscall.EPROTO:          EPROTO,
	syscall.EPROTONOSUPPORT: EPROTONOSUPPORT,
	syscall.EPROTOTYPE:      EPROTOTYPE,
	syscall.ERANGE:          ERANGE,
	syscall.EROFS:           EROFS,
	syscall.ESPIPE:          ESPIPE,
	syscall.ESRCH:           ESRCH,
	syscall.ESTALE:          ESTALE,
	syscall.ETIMEDOUT:       ETIMEDOUT,
	// syscall.ETXTBSY:         ETXTBSY,
	syscall.EXDEV: EXDEV,
}

var mapErrno2Syscall = map[errno]syscall.Errno{
	E2BIG:         syscall.E2BIG,
	EACCES:        syscall.EACCES,
	EADDRINUSE:    syscall.EADDRINUSE,
	EADDRNOTAVAIL: syscall.EADDRNOTAVAIL,
	EAFNOSUPPORT:  syscall.EAFNOSUPPORT,
	EAGAIN:        syscall.EAGAIN,
	EALREADY:      syscall.EALREADY,
	EBADF:         syscall.EBADF,
	EBADMSG:       syscall.EBADMSG,
	EBUSY:         syscall.EBUSY,
	ECANCELED:     syscall.ECANCELED,
	ECHILD:        syscall.ECHILD,
	ECONNABORTED:  syscall.ECONNABORTED,
	ECONNREFUSED:  syscall.ECONNREFUSED,
	ECONNRESET:    syscall.ECONNRESET,
	EDEADLK:       syscall.EDEADLK,
	EDESTADDRREQ:  syscall.EDESTADDRREQ,
	EDOM:          syscall.EDOM,
	EDQUOT:        syscall.EDQUOT,
	EEXIST:        syscall.EEXIST,
	EFAULT:        syscall.EFAULT,
	EFBIG:         syscall.EFBIG,
	EHOSTUNREACH:  syscall.EHOSTUNREACH,
	EIDRM:         syscall.EIDRM,
	EILSEQ:        syscall.EILSEQ,
	EINPROGRESS:   syscall.EINPROGRESS,
	EINTR:         syscall.EINTR,
	EINVAL:        syscall.EINVAL,
	EIO:           syscall.EIO,
	EISCONN:       syscall.EISCONN,
	EISDIR:        syscall.EISDIR,
	ELOOP:         syscall.ELOOP,
	EMFILE:        syscall.EMFILE,
	EMLINK:        syscall.EMLINK,
	EMSGSIZE:      syscall.EMSGSIZE,
	EMULTIHOP:     syscall.EMULTIHOP,
	ENAMETOOLONG:  syscall.ENAMETOOLONG,
	ENETDOWN:      syscall.ENETDOWN,
	ENETRESET:     syscall.ENETRESET,
	ENETUNREACH:   syscall.ENETUNREACH,
	ENFILE:        syscall.ENFILE,
	ENOBUFS:       syscall.ENOBUFS,
	ENODEV:        syscall.ENODEV,
	ENOENT:        syscall.ENOENT,
	ENOEXEC:       syscall.ENOEXEC,
	ENOLCK:        syscall.ENOLCK,
	ENOLINK:       syscall.ENOLINK,
	ENOMEM:        syscall.ENOMEM,
	ENOMSG:        syscall.ENOMSG,
	ENOPROTOOPT:   syscall.ENOPROTOOPT,
	ENOSPC:        syscall.ENOSPC,
	ENOSYS:        syscall.ENOSYS,
	ENOTCONN:      syscall.ENOTCONN,
	ENOTDIR:       syscall.ENOTDIR,
	ENOTEMPTY:     syscall.ENOTEMPTY,
	// ENOTRECOVERABLE: syscall.ENOTRECOVERABLE,
	ENOTSOCK:  syscall.ENOTSOCK,
	ENOTSUP:   syscall.ENOTSUP,
	ENOTTY:    syscall.ENOTTY,
	ENXIO:     syscall.ENXIO,
	EOVERFLOW: syscall.EOVERFLOW,
	// EOWNERDEAD:      syscall.EOWNERDEAD,
	EPERM:           syscall.EPERM,
	EPIPE:           syscall.EPIPE,
	EPROTO:          syscall.EPROTO,
	EPROTONOSUPPORT: syscall.EPROTONOSUPPORT,
	EPROTOTYPE:      syscall.EPROTOTYPE,
	ERANGE:          syscall.ERANGE,
	EROFS:           syscall.EROFS,
	ESPIPE:          syscall.ESPIPE,
	ESRCH:           syscall.ESRCH,
	ESTALE:          syscall.ESTALE,
	ETIMEDOUT:       syscall.ETIMEDOUT,
	// ETXTBSY:         syscall.ETXTBSY,
	EXDEV: syscall.EXDEV,
}
