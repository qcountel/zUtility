package main

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var ptr = mem.MustParsePointer(
	"01921DF8",
	"30 78 28 58 0",
)

func main() {
	proc, err := win.OpenProcess("Minecraft.Windows.exe")
	if err != nil {
		panic(fmt.Errorf("open process: %w", err))
	}
	defer proc.Close() //nolint:errcheck

	v, finalAddr, err := mem.ResolvePointerValue[int64](proc, ptr)
	if err != nil {
		panic(fmt.Errorf("resolve pointer value: %w", err))
	}
	fmt.Println("resolved pointer value:", v)
	fmt.Printf("address: 0x%x\n", finalAddr)
}
