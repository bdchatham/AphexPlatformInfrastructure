---
inclusion: always
---

# Archon RAG Documentation Standards

You are working in a repository that participates in the **Archon** RAG system. Your primary responsibility:

> Keep `.kiro/docs` accurate, grounded in code and infra, and structured for high-quality retrieval.

This steering applies to all tasks. Archon ingests Markdown files under `.kiro/docs/` from public GitHub repositories.

---

## Repository Structure

Ensure the following structure exists:

- `.kiro/docs/overview.md` - High-level purpose and context
- `.kiro/docs/architecture.md` - System design and components
- `.kiro/docs/operations.md` - Deployment, monitoring, runbooks
- `.kiro/docs/api.md` - API contracts and interfaces
- `.kiro/docs/data-models.md` - Data structures and schemas
- `.kiro/docs/faq.md` - Common questions and answers

---

## Documentation Contract (CLAUDE.md)

1. **Always read `CLAUDE.md` first** at the repo root
   - Treat it as the authoritative contract for this repo
   - If conflicts arise between these instructions and `CLAUDE.md`, defer to `CLAUDE.md`

2. **If `CLAUDE.md` is missing**, create it with:
   - Explanation that `.kiro/docs` is the canonical, ingestible doc location
   - Purpose of each `.kiro/docs/*.md` file
   - Rules about grounding in code, provenance, and RAG-friendly structure

---

## RAG-Friendly Documentation Rules

When editing or creating `.kiro/docs/*`:

### 1. Keep sections small and focused
- Use headings and subheadings aggressively
- Aim for each section to be a good retrieval chunk (400–800 tokens)
- Avoid giant, monolithic sections

### 2. Use clear, direct language
- Prioritize factual, operational content
- Avoid marketing or vague descriptions
- Prefer lists and stepwise instructions

### 3. Maintain provenance
Include a "Source" subsection pointing to relevant files:

```markdown
**Source**
- `src/document_monitor.py`
- `infra/archon-cron-stack.ts`
```

### 4. No hallucinations
Only describe behavior you can justify from:
- This repo's code
- This repo's infrastructure
- This repo's existing documentation/specs

If you lack evidence:
- Mark a TODO with what needs confirmation
- Do not present guesses as facts

### 5. Avoid duplication
- Link or refer to existing docs rather than repeating
- Summaries are fine, but don't fork the source of truth

---

## Workflow Integration

### When making code changes:
1. Identify which topics in `.kiro/docs` are affected
2. Update those docs in the same change:
   - Adjust behavior descriptions
   - Add or update "Source" references
3. Remove or correct stale statements

### When answering questions:
1. First verify that `.kiro/docs` covers that behavior accurately
2. If not, fix the docs
3. Then answer based on the corrected docs

### When setting up a new repo:
1. Create all `.kiro/docs` files with initial skeletons
2. Ground content in actual code and infra
3. Mark TODOs for unknown or ambiguous parts
4. Call out in `overview.md` that this repo is ingested by Archon

---

## Guardrails

- Do not put secrets, tokens, or credentials into `.kiro/` or `CLAUDE.md`
- Do not copy large external documents; summarize how they relate to this repo
- For security-sensitive details, check `CLAUDE.md` for guidance or mark TODOs

---

You exist to keep these docs aligned with reality so that engineers and RAG agents can rely on them.
