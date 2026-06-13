package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	grokpkg "github.com/grok-fireworks-reg/internal/grok"
	fwpkg "github.com/grok-fireworks-reg/internal/fireworks"
	orpkg "github.com/grok-fireworks-reg/internal/openrouter"
	nvpkg "github.com/grok-fireworks-reg/internal/novita"
)

// BlacklistEntry 黑名单条目（API 返回格式）
type BlacklistEntry struct {
	Domain   string `json:"domain"`
	BannedAt string `json:"banned_at"`
	Ago      string `json:"ago"` // "1h30m ago"
}

// GetGrokBlacklist GET /api/blacklist/grok
func GetGrokBlacklist(c *gin.Context) {
	entries := formatBlacklist(grokpkg.GetBlacklist().GetAll())
	c.JSON(http.StatusOK, gin.H{"platform": "grok", "domains": entries, "count": len(entries)})
}

// GetFireworksBlacklist GET /api/blacklist/fireworks
func GetFireworksBlacklist(c *gin.Context) {
	entries := formatBlacklist(fwpkg.GetBlacklist().GetAll())
	c.JSON(http.StatusOK, gin.H{"platform": "fireworks", "domains": entries, "count": len(entries)})
}

// ClearGrokBlacklist DELETE /api/blacklist/grok
func ClearGrokBlacklist(c *gin.Context) {
	grokpkg.GetBlacklist().Clear()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Grok blacklist cleared"})
}

// ClearFireworksBlacklist DELETE /api/blacklist/fireworks
func ClearFireworksBlacklist(c *gin.Context) {
	fwpkg.GetBlacklist().Clear()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Fireworks 黑名单已清空"})
}

// GetOpenRouterBlacklist GET /api/blacklist/openrouter
func GetOpenRouterBlacklist(c *gin.Context) {
	entries := formatBlacklist(orpkg.GetBlacklist().GetAll())
	c.JSON(http.StatusOK, gin.H{"platform": "openrouter", "domains": entries, "count": len(entries)})
}

// ClearOpenRouterBlacklist DELETE /api/blacklist/openrouter
func ClearOpenRouterBlacklist(c *gin.Context) {
	orpkg.GetBlacklist().Clear()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "OpenRouter blacklist cleared"})
}

// GetNovitaBlacklist GET /api/blacklist/novita
func GetNovitaBlacklist(c *gin.Context) {
	entries := formatBlacklist(nvpkg.GetBlacklist().GetAll())
	c.JSON(http.StatusOK, gin.H{"platform": "novita", "domains": entries, "count": len(entries)})
}

// ClearNovitaBlacklist DELETE /api/blacklist/novita
func ClearNovitaBlacklist(c *gin.Context) {
	nvpkg.GetBlacklist().Clear()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Novita blacklist cleared"})
}

func formatBlacklist(all map[string]time.Time) []BlacklistEntry {
	entries := make([]BlacklistEntry, 0, len(all))
	now := time.Now()
	for domain, bannedAt := range all {
		ago := now.Sub(bannedAt).Truncate(time.Minute)
		entries = append(entries, BlacklistEntry{
			Domain:   domain,
			BannedAt: bannedAt.Format(time.RFC3339),
			Ago:      ago.String(),
		})
	}
	return entries
}
