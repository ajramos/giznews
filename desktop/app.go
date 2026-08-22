package main

import (
	"context"
	"os/exec"

	gizdesktop "github.com/ajramos/giznews/pkg/desktop"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound backend. Its exported methods are callable from the
// frontend (window.go.main.App.*). It intentionally does NOT take
// context.Context parameters (Wails binding quirk); a fresh background context
// is supplied per call.
type App struct {
	api *gizdesktop.App
}

func bg() context.Context { return context.Background() }

// ---- Sources ----
func (a *App) ListSources() ([]*gizdesktop.SourceDTO, error) {
	return a.api.ListSources(bg())
}
func (a *App) AddSource(name, srcType, url, group string) (*gizdesktop.SourceDTO, error) {
	return a.api.AddSource(bg(), name, srcType, url, group)
}
func (a *App) SetSourceEnabled(id int64, enabled bool) error {
	return a.api.SetSourceEnabled(bg(), id, enabled)
}
func (a *App) DeleteSource(id int64) error {
	return a.api.DeleteSource(bg(), id)
}

// ---- Articles ----
func (a *App) ListArticles(opts gizdesktop.ListArticlesOptions) ([]*gizdesktop.ArticleDTO, error) {
	return a.api.ListArticles(bg(), opts)
}
func (a *App) ListInbox(limit int) ([]*gizdesktop.ArticleDTO, error) {
	return a.api.ListInbox(bg(), limit)
}
func (a *App) GetArticle(id int64) (*gizdesktop.ArticleDTO, error) {
	return a.api.GetArticle(bg(), id)
}
func (a *App) GetArticleContent(id int64) (*gizdesktop.ArticleDTO, error) {
	return a.api.GetArticleContent(bg(), id)
}
func (a *App) SetArticleStatus(id int64, status string) error {
	return a.api.SetArticleStatus(bg(), id, status)
}
func (a *App) SetArticleStarred(id int64, starred bool) error {
	return a.api.SetArticleStarred(bg(), id, starred)
}
func (a *App) SetArticleImportance(id int64, importance int) error {
	return a.api.SetArticleImportance(bg(), id, importance)
}

// ---- Pipeline ----
func (a *App) Fetch() (*gizdesktop.FetchResult, error) {
	return a.api.Fetch(bg())
}
func (a *App) Classify(limit int) (*gizdesktop.ClassifyResult, error) {
	return a.api.Classify(bg(), limit)
}
func (a *App) ClassifyArticles(ids []int64) (*gizdesktop.ClassifyResult, error) {
	return a.api.ClassifyArticles(bg(), ids)
}
func (a *App) ClassifyRules(limit int) (*gizdesktop.ClassifyResult, error) {
	return a.api.ClassifyRules(bg(), limit)
}
func (a *App) SummarizeArticle(id int64) (*gizdesktop.ArticleDTO, error) {
	return a.api.SummarizeArticle(bg(), id)
}
func (a *App) Digest() (*gizdesktop.DigestDTO, error) {
	return a.api.Digest(bg())
}
func (a *App) ListDigests() ([]*gizdesktop.DigestMeta, error) {
	return a.api.ListDigests(bg())
}
func (a *App) GetDigest(date string) (*gizdesktop.DigestDTO, error) {
	return a.api.GetDigest(bg(), date)
}
func (a *App) Flow() (*gizdesktop.FlowStatus, error) {
	return a.api.Flow(bg())
}
func (a *App) Logs(limit int) string {
	return a.api.Logs(limit)
}

// ---- Rules ----
func (a *App) ListRules() ([]*gizdesktop.RuleDTO, error) {
	return a.api.ListRules(bg())
}
func (a *App) AddRule(name, query string, actions []gizdesktop.RuleActionDTO, enabled bool) (*gizdesktop.RuleDTO, error) {
	return a.api.AddRule(bg(), name, query, actions, enabled)
}
func (a *App) UpdateRule(id int64, name, query string, actions []gizdesktop.RuleActionDTO, enabled bool) (*gizdesktop.RuleDTO, error) {
	return a.api.UpdateRule(bg(), id, name, query, actions, enabled)
}
func (a *App) SetRuleEnabled(id int64, enabled bool) error {
	return a.api.SetRuleEnabled(bg(), id, enabled)
}
func (a *App) DeleteRule(id int64) error {
	return a.api.DeleteRule(bg(), id)
}

// ---- Knowledge graph ----
func (a *App) KBuild() (*gizdesktop.KBResult, error) {
	return a.api.KBuild(bg())
}
func (a *App) KThemes() (*gizdesktop.KBResult, error) {
	return a.api.KThemes(bg())
}
func (a *App) KSynthesize(category string) (*gizdesktop.KBResult, error) {
	return a.api.KSynthesize(bg(), category)
}
func (a *App) Ask(question string) (*gizdesktop.AnswerDTO, error) {
	return a.api.Ask(bg(), question)
}
func (a *App) EnsureArticleNote(articleID int64) (*gizdesktop.NoteDTO, error) {
	return a.api.EnsureArticleNote(bg(), articleID)
}
func (a *App) GetArticleNote(articleID int64) (*gizdesktop.NoteDTO, error) {
	return a.api.GetArticleNote(bg(), articleID)
}
func (a *App) ListConcepts() ([]*gizdesktop.ConceptDTO, error) {
	return a.api.ListConcepts(bg())
}

func (a *App) PromoteConcept(slug string) (*gizdesktop.NoteDTO, error) {
	return a.api.PromoteConcept(bg(), slug)
}

func (a *App) MergeConcepts(from, to string) (*gizdesktop.MergeDTO, error) {
	return a.api.MergeConcepts(bg(), from, to)
}

func (a *App) ListNotes(noteType string) ([]*gizdesktop.NoteDTO, error) {
	return a.api.ListNotes(bg(), noteType)
}
func (a *App) GetNote(id int64) (*gizdesktop.NoteDTO, error) {
	return a.api.GetNote(bg(), id)
}
func (a *App) GraphNeighbors(id int64) ([]*gizdesktop.NoteDTO, error) {
	return a.api.GraphNeighbors(bg(), id)
}

// ---- Search ----
func (a *App) SearchIndex() (*gizdesktop.IndexResultDTO, error) {
	return a.api.SearchIndex(bg())
}
func (a *App) Search(query string, limit int) ([]*gizdesktop.SearchResultDTO, error) {
	return a.api.Search(bg(), query, limit)
}

// ---- Jobs ----
func (a *App) ListJobs() ([]*gizdesktop.JobDTO, error) {
	return a.api.ListJobs(bg())
}
func (a *App) RemoveJob(id int64) error {
	return a.api.RemoveJob(bg(), id)
}
func (a *App) ClearFinishedJobs() error {
	return a.api.ClearFinishedJobs(bg())
}
func (a *App) CancelJob(id int64) error {
	return a.api.CancelJob(bg(), id)
}
func (a *App) BulkSetStatus(ids []int64, status string) (*gizdesktop.BulkResult, error) {
	return a.api.BulkSetStatus(bg(), ids, status)
}
func (a *App) IngestURL(url string) (*gizdesktop.ArticleDTO, error) {
	return a.api.IngestURL(bg(), url)
}

// ---- Meta ----
func (a *App) Status() (*gizdesktop.StatusDTO, error) {
	return a.api.Status(bg())
}

// ---- Actions ----
func (a *App) OpenVault() error {
	return exec.Command("open", a.api.VaultPath()).Start()
}
func (a *App) OpenURL(raw string) error {
	if raw == "" {
		return nil
	}
	return exec.Command("open", raw).Start()
}
func (a *App) Quit() {
	runtime.Quit(bg())
}
