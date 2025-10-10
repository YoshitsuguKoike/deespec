package version

import (
	"fmt"
	"runtime"

	"github.com/YoshitsuguKoike/deespec/internal/buildinfo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version, build information, and runtime details",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("deespec version %s\n", buildinfo.GetVersion())
			fmt.Printf("  Go version:    %s\n", runtime.Version())
			fmt.Printf("  OS/Arch:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Printf("  Compiler:      %s\n", runtime.Compiler)
		},
	}
}
