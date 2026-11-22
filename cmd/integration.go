package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hongnoul/mangaz/icon"
	"github.com/hongnoul/mangaz/integration/anilist"
	"github.com/hongnoul/mangaz/key"
	"github.com/hongnoul/mangaz/log"
	"github.com/hongnoul/mangaz/open"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(integrationCmd)
	integrationCmd.AddCommand(integrationAnilistCmd)
	integrationAnilistCmd.Flags().BoolP("disable", "d", false, "Log out and disable Anilist integration")
	integrationAnilistCmd.Flags().Bool("no-browser", false, "Print the login URL instead of opening a browser")
	integrationAnilistCmd.Flags().String("client-id", "", "Use your own Anilist API client (redirect URL "+anilist.RedirectURL+")")
}

var integrationCmd = &cobra.Command{
	Use:   "integration",
	Short: "Integration with other services",
	Long:  `Integration with other services`,
}

var integrationAnilistCmd = &cobra.Command{
	Use:   "anilist",
	Short: "Log in to Anilist to sync reading progress",
	Long: `Log in to Anilist. Your browser opens, you click "Authorize", and you're done.
Chapters you read are then marked as read on your Anilist list.

On a machine without a browser (SSH), use --no-browser: open the printed URL
anywhere, approve, then paste the address bar URL you land on.`,
	Run: func(cmd *cobra.Command, args []string) {
		if lo.Must(cmd.Flags().GetBool("disable")) {
			for _, k := range []string{key.AnilistToken, key.AnilistCode, key.AnilistSecret, key.AnilistID} {
				viper.Set(k, "")
			}
			viper.Set(key.AnilistEnable, false)
			handleErr(writeConfig())
			fmt.Printf("%s Logged out of Anilist\n", icon.Get(icon.Success))
			return
		}

		if id := lo.Must(cmd.Flags().GetString("client-id")); id != "" {
			viper.Set(key.AnilistID, id)
		}

		clientID := anilist.ClientID()
		if clientID == "" {
			handleErr(fmt.Errorf("this build has no bundled Anilist client. Create one at "+
				"https://anilist.co/settings/developer with redirect URL %s, then run "+
				"`mangaz integration anilist --client-id <id>`", anilist.RedirectURL))
		}

		authURL := anilist.ImplicitAuthURL(clientID)
		noBrowser := lo.Must(cmd.Flags().GetBool("no-browser"))

		token, err := login(authURL, noBrowser)
		handleErr(err)

		name, err := anilist.Viewer(token.AccessToken)
		handleErr(err)

		viper.Set(key.AnilistToken, token.AccessToken)
		viper.Set(key.AnilistEnable, true)
		viper.Set(key.AnilistCode, "")
		viper.Set(key.AnilistSecret, "")
		handleErr(writeConfig())

		fmt.Printf("%s Logged in to Anilist as %s\n", icon.Get(icon.Success), name)
	},
}

// login opens the browser and waits for the localhost callback. Pasting the
// redirected URL into the terminal also works, which covers SSH sessions and
// a busy callback port.
func login(authURL string, noBrowser bool) (anilist.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	type result struct {
		token anilist.Token
		err   error
	}
	results := make(chan result, 2)

	go func() {
		t, err := anilist.ListenForToken(ctx)
		if err != nil && ctx.Err() == nil {
			// Port busy: pasting the redirected URL still works.
			log.Warn(err.Error())
			return
		}
		results <- result{t, err}
	}()

	opened := false
	if !noBrowser {
		opened = open.Start(authURL) == nil
	}

	if opened {
		fmt.Println("Opened your browser. Click \"Authorize\" to finish logging in.")
		fmt.Println("If nothing opened, visit:")
	} else {
		fmt.Println("Open this URL in a browser and click \"Authorize\":")
	}
	fmt.Println("  " + authURL)
	fmt.Println()
	fmt.Println("Waiting... (or paste the URL you were redirected to and press Enter)")

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadString('\n')
			if strings.TrimSpace(line) != "" {
				t, perr := anilist.ParseCallback(line)
				if perr != nil {
					fmt.Println("Couldn't find a token in that:", perr)
					continue
				}
				results <- result{t, nil}
				return
			}
			if err != nil {
				return
			}
		}
	}()

	select {
	case r := <-results:
		return r.token, r.err
	case <-ctx.Done():
		return anilist.Token{}, fmt.Errorf("timed out waiting for Anilist authorization")
	}
}

func writeConfig() error {
	err := viper.WriteConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return viper.SafeWriteConfig()
	}
	return err
}
