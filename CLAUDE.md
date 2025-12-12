# Archon Documentation Contract

This repository participates in the **Archon** RAG system, which ingests documentation to build mental models for sourcing code and architectural information.

## Documentation Location

The canonical, ingestible documentation location is **`.kiro/docs/`**.

Archon ingests all Markdown files under `.kiro/docs/` from this public GitHub repository.

## Required Documentation Files

Maintain the following files under `.kiro/docs/`:

### `.kiro/docs/overview.md`
High-level purpose, context, and scope of this repository. Explains what problem this repo solves and how it fits into the broader system.

### `.kiro/docs/architecture.md`
System design, components, and their relationships. Includes diagrams (as Markdown), technology choices, and architectural patterns.

### `.kiro/docs/operations.md`
Deployment procedures, monitoring, alerting, runbooks, and operational concerns. How to deploy, troubleshoot, and maintain this system.

### `.kiro/docs/api.md`
API contracts, interfaces, endpoints, and integration patterns. Documents how other systems interact with this one.

### `.kiro/docs/data-models.md`
Data structures, schemas, database models, and data flow. Describes what data this system manages and how.

### `.kiro/docs/faq.md`
Common questions, gotchas, and quick answers. Helps new contributors and operators get up to speed.

## Documentation Standards

### Grounding in Code
All documentation must be grounded in actual code and infrastructure:
- Reference specific files (e.g., `src/handler.py`, `infra/stack.ts`)
- Include "Source" sections pointing to relevant code
- Update docs when code changes

### RAG-Friendly Structure
Documentation should be optimized for retrieval:
- Use clear headings and subheadings
- Keep sections focused (400–800 tokens each)
- Use direct, factual language
- Prefer lists and step-by-step instructions
- Avoid long, monolithic sections

### Provenance
Each significant section should include a "Source" subsection:

```markdown
**Source**
- `src/document_monitor.py`
- `infra/archon-cron-stack.ts`
```

### No Hallucinations
Only document behavior that can be verified from:
- This repository's code
- This repository's infrastructure
- This repository's existing specs

If something is uncertain, mark it as a TODO rather than guessing.

### Avoid Duplication
Link to existing documentation rather than repeating it. Maintain a single source of truth for each concept.

## Security

Do not include:
- Secrets, tokens, or credentials
- Sensitive internal details without review
- Large external documents (summarize instead)

## Kiro Integration

This repository includes Kiro steering at `.kiro/steering/archon-docs.md` that enforces these standards automatically across all Kiro tasks.

When working with Kiro:
- Documentation updates should accompany code changes
- Kiro will help maintain documentation accuracy
- Kiro will ensure RAG-friendly structure

## Contract Authority

This `CLAUDE.md` file is the authoritative contract for this repository. If conflicts arise between this contract and other instructions, this contract takes precedence.
