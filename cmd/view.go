package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/hongnoul/mangaz/reader"
	"github.com/hongnoul/mangaz/reader/render"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(viewCmd)
	viewCmd.Flags().IntP("page", "p", 1, "page to open at (1-based)")
	viewCmd.Flags().String("tone", "dark", "page tone: dark, light, dim")
	viewCmd.Flags().Bool("ltr", false, "read left-to-right (default is right-to-left)")
	viewCmd.Flags().Int("lanes", 0, "render worker count (0 = auto)")
}

var viewCmd = &cobra.Command{
	Use:   "view <dir|cbz>",
	Short: "Read a downloaded chapter in the terminal",
	Long: `Read a chapter (a folder of page images, or a .cbz/.zip) inside the
terminal using the kitty graphics protocol. Needs kitty, ghostty, wezterm,
or gwae.

Keys:
  j / space / down   next page        k / up        previous page
  h / left           page to the left l / right     page to the right
                     (right-to-left: left is the next page)
  g / G              first / last     f <digits>    go to page
  s                  two-page spread  D             flip reading direction
  d                  tone: dark, light, dim
  r / R              rotate clockwise / counterclockwise
  q / ctrl-c         quit

MANGAZ_TRACE=1 writes timing lines to stderr.`,
	Args:    cobra.ExactArgs(1),
	Example: "  mangaz view ./Chainsaw_Man/ch1\n  mangaz view chapter.cbz --page 5",
	Run: func(cmd *cobra.Command, args []string) {
		opt := reader.Options{
			Start: lo.Must(cmd.Flags().GetInt("page")) - 1,
			Lanes: lo.Must(cmd.Flags().GetInt("lanes")),
		}
		switch strings.ToLower(lo.Must(cmd.Flags().GetString("tone"))) {
		case "dark":
		case "light":
			t := render.Light
			opt.Tone = &t
		case "dim":
			t := render.Dim
			opt.Tone = &t
		default:
			handleErr(fmt.Errorf("unknown tone %q (dark, light, dim)", lo.Must(cmd.Flags().GetString("tone"))))
		}
		if lo.Must(cmd.Flags().GetBool("ltr")) {
			rtl := false
			opt.RTL = &rtl
		}
		if os.Getenv("MANGAZ_TRACE") == "1" {
			opt.Trace = os.Stderr
		}
		_, err := reader.Open(args[0], opt)
		handleErr(err)
	},
}
