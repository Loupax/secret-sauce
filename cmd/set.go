package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var interactive bool

var setCmd = &cobra.Command{
	Use:   "set <type> <key> [value | k=v ...]",
	Short: "Set a secret",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && args[0] == "map" && interactive {
			if len(args) != 2 {
				return fmt.Errorf("interactive map set accepts 2 arg(s), received %d", len(args))
			}
			return nil
		}
		if args[0] == "map" {
			// map type: type + key + one or more k=v pairs
			if len(args) < 3 {
				return fmt.Errorf("map set requires at least 3 args (type key k=v ...), received %d", len(args))
			}
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
		subtype := args[0]
		if subtype != "environment" && subtype != "file" && subtype != "map" {
			return fmt.Errorf("type must be 'environment', 'file', or 'map'; got %q", subtype)
		}
		key := args[1]

		var data map[string]string

		switch subtype {
		case "map":
			if interactive {
				data = make(map[string]string)
				reader := bufio.NewReader(os.Stdin)
				for {
					fmt.Fprint(os.Stderr, "key (empty to finish): ")
					k, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("read key: %w", err)
					}
					k = strings.TrimRight(k, "\r\n")
					if k == "" {
						break
					}
					fmt.Fprintf(os.Stderr, "value for %q: ", k)
					v, err := term.ReadPassword(int(os.Stdin.Fd()))
					if err != nil {
						return fmt.Errorf("read value: %w", err)
					}
					fmt.Fprintln(os.Stderr)
					data[k] = string(v)
				}
			} else {
				// args[2:] are k=v pairs
				data = make(map[string]string, len(args)-2)
				for _, pair := range args[2:] {
					idx := strings.IndexByte(pair, '=')
					if idx < 0 {
						return fmt.Errorf("invalid key=value pair %q: missing '='", pair)
					}
					data[pair[:idx]] = pair[idx+1:]
				}
			}

		default: // environment or file
			data = map[string]string{"value": args[2]}
		}

		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		if err := svc.WriteSecret(vaultDir, key, data); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}

		return nil
	},
}

func init() {
	setCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactively build a map secret key-by-key")
}
