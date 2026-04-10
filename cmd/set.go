package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/loupax/secret-sauce/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var interactive bool

var setCmd = &cobra.Command{
	Use:   "set <type> <key> <value>",
	Short: "Set a secret",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 2 && vault.SecretType(args[0]) == vault.SecretTypeMap && interactive {
			return nil
		}
		if len(args) != 3 {
			return fmt.Errorf("accepts 3 arg(s), received %d", len(args))
		}
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return []string{"environment", "file", "map"}, cobra.ShellCompDirectiveNoFileComp
		case 1:
			return completeSecretKeys(cmd, args, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		secretType := vault.SecretType(args[0])
		if !vault.ValidSecretType(secretType) {
			return fmt.Errorf("type must be 'environment', 'file', or 'map'; got %q", args[0])
		}
		key := args[1]

		var value string

		if secretType == vault.SecretTypeMap {
			if interactive {
				m := make(map[string]string)
				reader := bufio.NewReader(os.Stdin)
				for {
					fmt.Fprint(os.Stderr, "key (empty to finish): ")
					k, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("read key: %w", err)
					}
					k = k[:len(k)-1]
					if len(k) > 0 && k[len(k)-1] == '\r' {
						k = k[:len(k)-1]
					}
					if k == "" {
						break
					}
					fmt.Fprintf(os.Stderr, "value for %q: ", k)
					v, err := term.ReadPassword(int(os.Stdin.Fd()))
					if err != nil {
						return fmt.Errorf("read value: %w", err)
					}
					fmt.Fprintln(os.Stderr)
					m[k] = string(v)
				}
				b, _ := json.Marshal(m)
				value = string(b)
			} else {
				var raw map[string]interface{}
				if err := json.Unmarshal([]byte(args[2]), &raw); err != nil {
					return fmt.Errorf("invalid JSON: %w", err)
				}
				for k, v := range raw {
					if _, ok := v.(string); !ok {
						return fmt.Errorf("map values must be strings; key %q has a non-string value", k)
					}
				}
				b, _ := json.Marshal(raw)
				value = string(b)
			}
		} else {
			value = args[2]
		}

		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		if err := svc.WriteSecret(vaultDir, key, value, secretType); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}

		return nil
	},
}

func init() {
	setCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactively build a map secret key-by-key")
}
