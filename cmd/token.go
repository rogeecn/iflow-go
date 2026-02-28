package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
	"github.com/rogeecn/iflow-go/internal/config"
	"github.com/rogeecn/iflow-go/internal/oauth"
	"github.com/spf13/cobra"
)

type oauthClient interface {
	Login(ctx context.Context) (*account.Account, error)
	Refresh(ctx context.Context, refreshToken string) (*oauth.Token, error)
}

var newOAuthClient = func(manager *account.Manager) oauthClient {
	return oauth.NewClientWithManager(manager)
}

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Token 管理",
}

var tokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有账号",
	RunE:  runTokenList,
}

var tokenImportCmd = &cobra.Command{
	Use:   "import [settings.json]",
	Short: "导入账号 (OAuth 登录或从 settings.json 导入)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTokenImport,
}

var tokenDeleteCmd = &cobra.Command{
	Use:   "delete <uuid>",
	Short: "删除账号",
	Args:  cobra.ExactArgs(1),
	RunE:  runTokenDelete,
}

var tokenRefreshCmd = &cobra.Command{
	Use:   "refresh <uuid>",
	Short: "手动刷新 Token",
	Args:  cobra.ExactArgs(1),
	RunE:  runTokenRefresh,
}

func init() {
	rootCmd.AddCommand(tokenCmd)
	tokenCmd.AddCommand(tokenListCmd)
	tokenCmd.AddCommand(tokenImportCmd)
	tokenCmd.AddCommand(tokenDeleteCmd)
	tokenCmd.AddCommand(tokenRefreshCmd)
}

func runTokenList(cmd *cobra.Command, _ []string) error {
	manager, err := newAccountManager()
	if err != nil {
		return err
	}

	accounts, err := manager.List()
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}

	if len(accounts) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No accounts found.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "UUID\tAUTH\tREQUESTS\tUPDATED_AT")
	for _, acct := range accounts {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\t%s\n",
			acct.UUID,
			acct.AuthType,
			acct.RequestCount,
			acct.UpdatedAt.Format(time.RFC3339),
		)
	}
	return nil
}

func runTokenImport(cmd *cobra.Command, args []string) error {
	manager, err := newAccountManager()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		acct, err := importFromSettingsFile(manager, args[0])
		if err != nil {
			return fmt.Errorf("settings import: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Account imported successfully.\nUUID: %s\n", acct.UUID)
		return nil
	}

	client := newOAuthClient(manager)
	acct, err := client.Login(context.Background())
	if err != nil {
		return fmt.Errorf("oauth import: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Account imported successfully.\nUUID: %s\n", acct.UUID)
	return nil
}

func runTokenDelete(cmd *cobra.Command, args []string) error {
	uuid := strings.TrimSpace(args[0])
	if !account.IsValidUUID(uuid) {
		return fmt.Errorf("invalid uuid: %s", uuid)
	}

	manager, err := newAccountManager()
	if err != nil {
		return err
	}

	if err := manager.Delete(uuid); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Account deleted: %s\n", uuid)
	return nil
}

func runTokenRefresh(cmd *cobra.Command, args []string) error {
	uuid := strings.TrimSpace(args[0])
	if !account.IsValidUUID(uuid) {
		return fmt.Errorf("invalid uuid: %s", uuid)
	}

	manager, err := newAccountManager()
	if err != nil {
		return err
	}

	acct, err := manager.Get(uuid)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}

	refreshToken := strings.TrimSpace(acct.OAuthRefreshToken)
	if refreshToken == "" {
		return fmt.Errorf("account %s has no refresh token", uuid)
	}

	client := newOAuthClient(manager)
	token, err := client.Refresh(context.Background(), refreshToken)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}

	newRefreshToken := strings.TrimSpace(token.RefreshToken)
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	expiresAt := token.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(24 * time.Hour)
	}

	if err := manager.UpdateToken(uuid, token.AccessToken, newRefreshToken, expiresAt); err != nil {
		return fmt.Errorf("persist refreshed token: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Token refreshed: %s\n", uuid)
	return nil
}

func newAccountManager() (*account.Manager, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return account.NewManager(cfg.DataDir), nil
}

type iflowSettingsFile struct {
	APIKey       string `json:"apiKey"`
	SearchAPIKey string `json:"searchApiKey"`
	BaseURL      string `json:"baseUrl"`
}

type iflowOAuthCredsFile struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiryDate   int64  `json:"expiry_date"`
}

func importFromSettingsFile(manager *account.Manager, path string) (*account.Account, error) {
	resolvedPath, err := resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	raw, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("read settings file: %w", err)
	}

	var settings iflowSettingsFile
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("parse settings json: %w", err)
	}

	apiKey := strings.TrimSpace(settings.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(settings.SearchAPIKey)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("settings file missing api key")
	}

	acct, err := manager.Create(apiKey, strings.TrimSpace(settings.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	credsPath := filepath.Join(filepath.Dir(resolvedPath), "oauth_creds.json")
	credsRaw, err := os.ReadFile(credsPath)
	if err == nil {
		var creds iflowOAuthCredsFile
		if jsonErr := json.Unmarshal(credsRaw, &creds); jsonErr == nil {
			access := strings.TrimSpace(creds.AccessToken)
			refresh := strings.TrimSpace(creds.RefreshToken)
			if access != "" || refresh != "" {
				expiresAt := time.Time{}
				if creds.ExpiryDate > 0 {
					expiresAt = time.UnixMilli(creds.ExpiryDate).UTC()
				}
				if updateErr := manager.UpdateToken(acct.UUID, access, refresh, expiresAt); updateErr != nil {
					return nil, fmt.Errorf("persist oauth creds: %w", updateErr)
				}
			}
		}
	}

	stored, err := manager.Get(acct.UUID)
	if err != nil {
		return nil, fmt.Errorf("reload account: %w", err)
	}

	return stored, nil
}

func resolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}

	return filepath.Abs(path)
}
