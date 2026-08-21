import { useEffect, useRef } from "react";
import { Inbox, Archive, Star, Search, X, Sparkles } from "lucide-react";
import type { ArticleDTO } from "../types";
import { stars, timeAgo, catClass } from "./Markdown";
import { CATEGORIES } from "./CategoryPicker";

interface Props {
  articles: ArticleDTO[];
  selectedIndex: number;
  loading: boolean;
  archived: boolean;
  starredFilter: boolean | null;
  readFilter: "all" | "unread" | "read";
  hasSources: boolean;
  bulk: boolean;
  bulkSel: Set<number>;
  unreadCount: number;
  filterCategory: string | null;
  importanceExact: number | null;
  filterUnclassified: boolean;
  summarizedFilter: boolean;
  filterQuery: string;
  onActive: () => void;
  onArchived: () => void;
  onStarred: () => void;
  onReadFilter: (v: "all" | "unread" | "read") => void;
  onCategory: (c: string | null) => void;
  onImportance: (n: number | null) => void;
  onUnclassified: (v: boolean) => void;
  onSummarized: (v: boolean) => void;
  onQuery: (q: string) => void;
  onToggleBulk: (id: number) => void;
  onSelect: (index: number) => void;
}

export function ArticleList({ articles, selectedIndex, loading, archived, starredFilter, readFilter, hasSources, bulk, bulkSel, unreadCount, filterCategory, importanceExact, filterUnclassified, summarizedFilter, filterQuery, onActive, onArchived, onStarred, onReadFilter, onCategory, onImportance, onUnclassified, onSummarized, onQuery, onToggleBulk, onSelect }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const lastSelected = useRef(selectedIndex);

  // Follow the cursor, but only when the cursor moved. The articles array gets
  // a new identity whenever anything about a row changes — a status flip, a
  // finished classify job — and scrolling then would yank the list back under
  // a reader who is scrolling it by hand.
  useEffect(() => {
    if (lastSelected.current === selectedIndex) return;
    lastSelected.current = selectedIndex;
    containerRef.current?.querySelector(".article-row.selected")?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex, articles]);

  const active = !archived && starredFilter == null;

  return (
    <div className="list-pane">
      <div className="list-head">
        <div className="view-tabs">
          <button
            className={`view-tab ${active ? "active" : ""}`}
            onClick={onActive}
            title="Unread + read"
          >
            <Inbox size={12} /> Active{unreadCount > 0 && <span className="view-count">{unreadCount}</span>}
          </button>
          <button
            className={`view-tab ${archived ? "active" : ""}`}
            onClick={onArchived}
            title="Archived (x)"
          >
            <Archive size={12} /> Archived
          </button>
          <button
            className={`view-tab ${starredFilter === true ? "active" : ""}`}
            onClick={onStarred}
            title="Starred (*)"
          >
            <Star size={12} /> Starred
          </button>
        </div>
        {active && (
          <div className="read-filter" title="Filter by read state">
            <button className={readFilter === "all" ? "active" : ""} onClick={() => onReadFilter("all")}>All</button>
            <button className={readFilter === "unread" ? "active" : ""} onClick={() => onReadFilter("unread")}>Unread</button>
            <button className={readFilter === "read" ? "active" : ""} onClick={() => onReadFilter("read")}>Read</button>
          </div>
        )}
      </div>

      <div className="filter-row">
        <div className="filter-chips">
          <button className={`chip ${filterCategory === null && !filterUnclassified ? "active" : ""}`} onClick={() => { onCategory(null); onUnclassified(false); }}>
            All
          </button>
          <button className={`chip ${filterUnclassified ? "active" : ""}`} onClick={() => onUnclassified(!filterUnclassified)}>
            Unclassified
          </button>
          <button className={`chip ${summarizedFilter ? "active" : ""}`} onClick={() => onSummarized(!summarizedFilter)}>
            <Sparkles size={11} /> Summarized
          </button>
          {CATEGORIES.map((c) => (
            <button
              key={c}
              className={`chip ${filterCategory === c ? "active" : ""}`}
              onClick={() => { onCategory(filterCategory === c ? null : c); onUnclassified(false); }}
            >
              {c}
            </button>
          ))}
        </div>
        <div className="filter-imp">
          <button
            className={`chip ${importanceExact == null ? "active" : ""}`}
            onClick={() => onImportance(null)}
            title="Any importance"
          >
            ★
          </button>
          {[0, 1, 2, 3].map((n) => (
            <button
              key={n}
              className={`chip ${importanceExact === n ? "active" : ""}`}
              onClick={() => onImportance(importanceExact === n ? null : n)}
              title={n === 0 ? "Importance = 0" : `Importance = ${n}`}
            >
              {n === 0 ? "0★" : `${n}★`}
            </button>
          ))}
        </div>
      </div>

      <div className="list-search">
        <Search size={13} />
        <input value={filterQuery} onChange={(e) => onQuery(e.target.value)} placeholder="filter by title or author…" />
        {filterQuery && <button className="icon-btn" onClick={() => onQuery("")} title="Clear"><X size={13} /></button>}
      </div>

      {loading ? (
        <div className="skeleton">
          {[...Array(8)].map((_, i) => <div key={i} className="bar" style={{ width: `${70 + (i % 3) * 12}%` }} />)}
        </div>
      ) : articles.length === 0 ? (
        <div className="empty with-icon">
          <Inbox size={30} />
          <span>{hasSources ? "No articles in this view." : "No sources configured."}</span>
          <span className="muted">{hasSources ? "Run :process to fetch and classify" : "Add sources with :sources then run :process"}</span>
        </div>
      ) : (
        <>
          <div className="article-list" ref={containerRef}>
            {articles.map((a, i) => {
              const inBulk = bulkSel.has(a.id);
              return (
              <div
                key={a.id}
                className={`article-row ${i === selectedIndex ? "selected" : ""} ${inBulk ? "bulk" : ""} ${a.status === "read" ? "read" : ""} ${a.status === "archived" ? "archived" : ""}`}
                onClick={() => (bulk ? onToggleBulk(a.id) : onSelect(i))}
                onDoubleClick={() => onSelect(i)}
              >
                {bulk ? (
                  <span className="bulk-check">{inBulk ? "✓" : ""}</span>
                ) : (
                  <span className="article-flag" title={statusTitle(a)}>
                    {a.status === "unread" && <span className="unread-badge" />}
                    {a.status === "read" && <span className="read-badge" />}
                    {a.starred === true && <span className="star-badge">★</span>}
                  </span>
                )}
                <span className="summary-flag" title={a.summary ? "AI summary" : undefined}>
                  {a.summary && <Sparkles size={12} />}
                </span>
                <span className="article-imp imp" data-level={a.importance} title={`Importance: ${a.importance}/3`}>
                  {stars(a.importance)}
                </span>
                <span className="article-main">
                  <span className="article-title">{a.title}</span>
                  <span className="article-meta">
                    {a.category && <span className={`cat-chip ${catClass(a.category)}`}>{a.category}</span>}
                    <span className="src">{a.sourceName}</span>
                    {(a.storySize ?? 0) > 1 && (
                      <span className="src-count" title={`Also covered by: ${(a.storySources ?? []).join(", ")}`}>
                        +{(a.storySize ?? 1) - 1} more
                      </span>
                    )}
                    <span className="time" title={a.published || a.fetchedAt}>{timeAgo(a.published || a.fetchedAt)}</span>
                  </span>
                </span>
              </div>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}

function statusTitle(a: ArticleDTO): string {
  const parts = [a.status === "archived" ? "Archived" : a.status === "read" ? "Read" : "Unread"];
  if (a.starred === true) parts.push("Starred");
  return parts.join(" · ");
}
