package main

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var ptr = mem.MustParsePointer(
	"019209F0",
	"0 8 0 28 90",
)

func main() {
	proc, err := win.OpenProcess("Minecraft.Windows.exe")
	if err != nil {
		panic(fmt.Errorf("open process: %w", err))
	}
	defer proc.Close() //nolint:errcheck

	v, finalAddr, err := mem.ResolvePointerValue[[200]byte](proc, ptr)
	if err != nil {
		panic(fmt.Errorf("resolve pointer value: %w", err))
	}
	fmt.Printf("resolved pointer value: %q\n", toStr(v))
	fmt.Printf("address: 0x%x\n", finalAddr)
}

func toStr(x [200]byte) (s string) {
	for _, v := range x[:] {
		if v == 0 {
			break
		}
		s += string(v)
	}
	return
}
