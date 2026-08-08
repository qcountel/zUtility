package win

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	w "golang.org/x/sys/windows"
)

func ParseVersion(s string) (ProcessVersion, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ProcessVersion{}, errors.New("empty string")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 4 {
		return ProcessVersion{}, errors.New("too many parts")
	}
	nums := [4]uint32{}
	for i, part := range parts {
		val, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return ProcessVersion{}, fmt.Errorf("parse %q as uint: %w", part, err)
		}
		nums[i] = uint32(val)
	}
	return ProcessVersion{
		Major: nums[0],
		Minor: nums[1],
		Patch: nums[2],
		Build: nums[3],
	}, nil
}

func MustParseVersion(s string) ProcessVersion {
	v, err := ParseVersion(s)
	if err != nil {
		panic(fmt.Errorf("win: MustParseVersion: %w", err))
	}
	return v
}

type ProcessVersion struct {
	Major, Minor, Patch, Build uint32
}

func (v ProcessVersion) Zero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0 && v.Build == 0
}

func (v ProcessVersion) String() string {
	if v.Build != 0 {
		return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
	}
	if v.Patch != 0 {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	if v.Minor != 0 {
		return fmt.Sprintf("%d.%d", v.Major, v.Minor)
	}
	return fmt.Sprintf("%d", v.Major)
}

func (v ProcessVersion) ValueB() int64 {
	return (int64(v.Major) << 48) | (int64(v.Minor) << 32) | (int64(v.Patch) << 16) | int64(v.Build)
}

func (v ProcessVersion) Value() int64 {
	return (int64(v.Major) << 32) | (int64(v.Minor) << 16) | int64(v.Patch)
}

func (v ProcessVersion) GT(other ProcessVersion) bool {
	return v.Value() > other.Value()
}
func (v ProcessVersion) GTE(other ProcessVersion) bool {
	return v.Value() >= other.Value()
}
func (v ProcessVersion) LT(other ProcessVersion) bool {
	return v.Value() < other.Value()
}
func (v ProcessVersion) LTE(other ProcessVersion) bool {
	return v.Value() <= other.Value()
}
func (v ProcessVersion) E(other ProcessVersion) bool {
	return v.Value() == other.Value()
}

func (proc *Process) Version() ProcessVersion {
	return ProcessVersion{}
}

func GetProcessVersion(path string) (ProcessVersion, error) {
	size, err := w.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 {
		return ProcessVersion{}, fmt.Errorf("get file ver info size: %w", err)
	}
	buf := make([]byte, size)
	if err = w.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buf[0])); err != nil {
		return ProcessVersion{}, fmt.Errorf("get file ver info: %w", err)
	}
	info := new(w.VS_FIXEDFILEINFO)
	if err = w.VerQueryValue(unsafe.Pointer(&buf[0]), `\`, unsafe.Pointer(&info), new(uint32)); err != nil {
		return ProcessVersion{}, fmt.Errorf("ver query val: %w", err)
	}
	return ProcessVersion{
		Major: info.FileVersionMS >> 16,
		Minor: info.FileVersionMS & 0xFFFF,
		Patch: info.FileVersionLS >> 16,
		Build: info.FileVersionLS & 0xFFFF,
	}, nil
}
