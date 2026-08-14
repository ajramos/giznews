package desktop

import (
	"context"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/kb"
)

// kbService builds the knowledge-graph service from config.
func (a *App) kbService() (*kb.Service, error) {
	prov, err := a.provider()
	if err != nil {
		return nil, err
	}
	return kb.NewService(a.db, a.cfg.ResolveVaultPath(), kb.Options{
		ImportanceThreshold: a.cfg.Classify.ImportanceThreshold,
		Model:               a.cfg.LLM.Model,
		UseLLM:              a.cfg.LLM.Enabled && prov != nil,
	}, prov, discardLogger())
}

// KBResult is the desktop DTO for a kb build run.
type KBResult struct {
	AtomsCreated     int `json:"atoms_created"`
	ElectronsCreated int `json:"electrons_created"`
	ElectronsUpdated int `json:"electrons_updated"`
	MoleculesCreated int `json:"molecules_created"`
	ArticlesSkipped  int `json:"articles_skipped"`
}

// KBuild ingests pending articles into the knowledge graph.
func (a *App) KBuild(ctx context.Context) (*KBResult, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	res, err := svc.Build(ctx)
	if err != nil {
		return nil, err
	}
	return &KBResult{
		AtomsCreated:     res.AtomsCreated,
		ElectronsCreated: res.ElectronsCreated,
		ElectronsUpdated: res.ElectronsUpdated,
		MoleculesCreated: res.MoleculesCreated,
		ArticlesSkipped:  res.ArticlesSkipped,
	}, nil
}

// KSynthesize creates a molecule summarizing a category.
func (a *App) KSynthesize(ctx context.Context, category string) (*KBResult, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	res, err := svc.Synthesize(ctx, category)
	if err != nil {
		return nil, err
	}
	return &KBResult{MoleculesCreated: res.MoleculesCreated}, nil
}

func (a *App) ListNotes(ctx context.Context, noteType string) ([]*NoteDTO, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	notes, err := svc.ListNotes(ctx, noteType, 200)
	if err != nil {
		return nil, err
	}
	out := make([]*NoteDTO, 0, len(notes))
	for _, n := range notes {
		out = append(out, toNoteDTO(n))
	}
	return out, nil
}

func (a *App) GetNote(ctx context.Context, id int64) (*NoteDTO, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	n, err := svc.GetNote(ctx, id)
	if err != nil {
		return nil, err
	}
	return toNoteDTO(n), nil
}

func (a *App) GraphNeighbors(ctx context.Context, id int64) ([]*NoteDTO, error) {
	svc, err := a.kbService()
	if err != nil {
		return nil, err
	}
	notes, err := svc.GraphNeighbors(ctx, id, 50)
	if err != nil {
		return nil, err
	}
	out := make([]*NoteDTO, 0, len(notes))
	for _, n := range notes {
		out = append(out, toNoteDTO(n))
	}
	return out, nil
}

func toNoteDTO(n *db.KBNote) *NoteDTO {
	return &NoteDTO{
		ID: n.ID, Type: string(n.Type), Title: n.Title, Slug: n.Slug,
		Content: n.Content, Tags: n.Tags, Wikilinks: n.Wikilinks, CreatedAt: n.CreatedAt,
	}
}
