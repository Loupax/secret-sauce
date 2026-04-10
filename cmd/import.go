package cmd

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/loupax/secret-sauce/internal/config"
	"github.com/loupax/secret-sauce/internal/service"
	"github.com/loupax/secret-sauce/internal/vault"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// 1PUX JSON structs
// ---------------------------------------------------------------------------

type OnePUXExport struct {
	Accounts []OnePUXAccount `json:"accounts"`
}

type OnePUXAccount struct {
	Vaults []OnePUXVault `json:"vaults"`
}

type OnePUXVault struct {
	Items []OnePUXItem `json:"items"`
}

type OnePUXItem struct {
	UUID         string          `json:"uuid"`
	CategoryUUID string          `json:"categoryUuid"`
	Overview     OnePUXOverview  `json:"overview"`
	Details      OnePUXDetails   `json:"details"`
}

type OnePUXOverview struct {
	Title string `json:"title"`
}

type OnePUXDetails struct {
	LoginFields        []OnePUXLoginField `json:"loginFields"`
	Sections           []OnePUXSection    `json:"sections"`
	DocumentAttributes OnePUXDocAttrs     `json:"documentAttributes"`
}

type OnePUXLoginField struct {
	Designation string `json:"designation"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

type OnePUXSection struct {
	Fields []OnePUXField `json:"fields"`
}

type OnePUXField struct {
	Title string           `json:"title"`
	Value OnePUXFieldValue `json:"value"`
}

type OnePUXFieldValue struct {
	StringValue string `json:"stringValue"`
}

type OnePUXDocAttrs struct {
	FileName string `json:"fileName"`
	FileID   string `json:"fileId"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var (
	reNonAlnum     = regexp.MustCompile(`[^a-z0-9_]`)
	reMultiUnderscore = regexp.MustCompile(`_+`)
)

// normalizeKey cleans s into a safe vault key fragment. Returns "" when
// the result would be empty, ".", or ".."; the caller is responsible for
// substituting a fallback.
func normalizeKey(s string) string {
	s = strings.ToLower(s)
	for _, ch := range []string{" ", "/", "\\", "-"} {
		s = strings.ReplaceAll(s, ch, "_")
	}
	s = reNonAlnum.ReplaceAllString(s, "")
	s = reMultiUnderscore.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" || s == "." || s == ".." {
		return ""
	}
	return s
}

func resolveConcurrency(flagValue int, cfg *config.Config) int {
	if flagValue > 0 {
		return flagValue
	}
	if cfg.Concurrency > 0 {
		return cfg.Concurrency
	}
	return runtime.NumCPU()
}

// ---------------------------------------------------------------------------
// Write task + bounded write runner
// ---------------------------------------------------------------------------

type writeTask struct {
	key        string
	value      string
	secretType vault.SecretType
}

func runBoundedWrites(tasks []writeTask, concurrency int, svc service.VaultService, dir string) (imported, skipped int) {
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, t := range tasks {
		t := t
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			err := svc.WriteSecret(dir, t.key, t.value, t.secretType)
			mu.Lock()
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to write %q: %v\n", t.key, err)
				skipped++
			} else {
				imported++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return imported, skipped
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import secrets from an external source",
}

var flagConcurrency int

var importOnePWCmd = &cobra.Command{
	Use:   "1password <path>",
	Short: "Import secrets from a 1Password .1pux export file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "CAUTION: .1pux files are unencrypted. Delete the export file immediately after import.")

		cfg, _ := config.Load()
		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		conc := resolveConcurrency(flagConcurrency, cfg)

		// Open the .1pux zip archive
		zr, err := zip.OpenReader(args[0])
		if err != nil {
			return fmt.Errorf("open archive: %w", err)
		}
		defer zr.Close()

		// Find and parse export.data
		var export OnePUXExport
		var foundExportData bool
		for _, f := range zr.File {
			if f.Name != "export.data" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open export.data: %w", err)
			}
			err = json.NewDecoder(rc).Decode(&export)
			rc.Close()
			if err != nil {
				return fmt.Errorf("parse export.data: %w", err)
			}
			foundExportData = true
			break
		}
		if !foundExportData {
			return fmt.Errorf("export.data not found in archive")
		}

		// Build a lookup of zip entries by name for document file access
		zipEntries := make(map[string]*zip.File, len(zr.File))
		for _, f := range zr.File {
			zipEntries[f.Name] = f
		}

		var tasks []writeTask
		skippedBeforeWrite := 0

		for _, account := range export.Accounts {
			for _, v := range account.Vaults {
				for _, item := range v.Items {
					// Derive the base key from the item title
					baseKey := normalizeKey(item.Overview.Title)
					if baseKey == "" {
						// Fallback using first 8 chars of UUID
						uuidSuffix := item.UUID
						if len(uuidSuffix) > 8 {
							uuidSuffix = uuidSuffix[:8]
						}
						baseKey = "item_" + uuidSuffix
					}

					switch item.CategoryUUID {
					case "com.1password.category.login", "com.1password.category.password":
						primaryValue := ""
						// Prefer field with designation == "password"
						for _, f := range item.Details.LoginFields {
							if f.Designation == "password" && f.Value != "" {
								primaryValue = f.Value
								break
							}
						}
						// Fallback: first non-empty login field value
						if primaryValue == "" {
							for _, f := range item.Details.LoginFields {
								if f.Value != "" {
									primaryValue = f.Value
									break
								}
							}
						}
						if primaryValue == "" {
							fmt.Fprintf(os.Stderr, "warning: skipping %q — no usable password value\n", item.Overview.Title)
							skippedBeforeWrite++
							continue
						}
						tasks = append(tasks, writeTask{baseKey, primaryValue, vault.SecretTypeEnvironment})

					case "com.1password.category.document":
						fileID := item.Details.DocumentAttributes.FileID
						fileName := item.Details.DocumentAttributes.FileName

						entryPath := "files/" + fileID
						zipFile, ok := zipEntries[entryPath]
						if !ok {
							fmt.Fprintf(os.Stderr, "warning: skipping document %q — zip entry %q not found\n", item.Overview.Title, entryPath)
							skippedBeforeWrite++
							continue
						}

						rc, err := zipFile.Open()
						if err != nil {
							fmt.Fprintf(os.Stderr, "warning: skipping document %q — cannot open zip entry: %v\n", item.Overview.Title, err)
							skippedBeforeWrite++
							continue
						}
						var rawBuf []byte
						buf := make([]byte, 4096)
						for {
							n, readErr := rc.Read(buf)
							if n > 0 {
								rawBuf = append(rawBuf, buf[:n]...)
							}
							if readErr != nil {
								break
							}
						}
						rc.Close()

						value := base64.StdEncoding.EncodeToString(rawBuf)

						// Key: normalizeKey(fileName), fallback normalizeKey(title)
						key := normalizeKey(fileName)
						if key == "" {
							key = baseKey
						}
						tasks = append(tasks, writeTask{key, value, vault.SecretTypeFile})

					case "com.1password.category.database", "com.1password.category.server":
						fields := map[string]string{}
						counter := 0
						for _, sec := range item.Details.Sections {
							for _, f := range sec.Fields {
								fk := normalizeKey(f.Title)
								if fk == "" {
									continue
								}
								if _, exists := fields[fk]; exists {
									fk = fk + "_" + strconv.Itoa(counter)
									counter++
								}
								fields[fk] = f.Value.StringValue
							}
						}
						jsonBytes, _ := json.Marshal(fields)
						tasks = append(tasks, writeTask{baseKey, string(jsonBytes), vault.SecretTypeMap})

					default:
						// Scan loginFields then sections for first non-empty string value
						value := ""
						for _, f := range item.Details.LoginFields {
							if f.Value != "" {
								value = f.Value
								break
							}
						}
						if value == "" {
							for _, sec := range item.Details.Sections {
								for _, f := range sec.Fields {
									if f.Value.StringValue != "" {
										value = f.Value.StringValue
										break
									}
								}
								if value != "" {
									break
								}
							}
						}
						if value == "" {
							fmt.Fprintf(os.Stderr, "warning: skipping %q — no usable value\n", item.Overview.Title)
							skippedBeforeWrite++
							continue
						}
						tasks = append(tasks, writeTask{baseKey, value, vault.SecretTypeEnvironment})
					}
				}
			}
		}

		imported, skippedWrite := runBoundedWrites(tasks, conc, svc, vaultDir)
		totalSkipped := skippedBeforeWrite + skippedWrite

		fmt.Printf("Imported %d secrets. Skipped %d.\n", imported, totalSkipped)

		if totalSkipped > 0 {
			return fmt.Errorf("%d secrets could not be imported", totalSkipped)
		}
		return nil
	},
}

func init() {
	importOnePWCmd.Flags().IntVar(&flagConcurrency, "concurrency", 0, "max parallel writes (0 = auto)")
	importCmd.AddCommand(importOnePWCmd)
}
