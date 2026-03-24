package cmd

import "github.com/spf13/cobra"

// completeSecretKeys is a shared ValidArgsFunction that returns the names of
// all stored secrets for shell autocompletion.
func completeSecretKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	svc, err := resolveService()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	secrets, err := svc.ReadAllSecrets(vaultDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	return keys, cobra.ShellCompDirectiveNoFileComp
}
