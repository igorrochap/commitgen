---
name: code-review
description: Review the changes since a fixed point (commit, branch, tag, or merge-base) along two axes — Standards (repo conventions) and Spec (does it match the originating issue/PRD). Use when the user wants to review a branch, a PR, work-in-progress changes, or asks to "review since X".
---

Two-axis review of the diff between `HEAD` and a fixed point the user supplies:

- **Standards** — does the code conform to this repo's documented coding standards?
- **Spec** — does the code faithfully implement the originating issue / PRD / spec?

Both axes run as **parallel sub-agents** so they don't pollute each other's context, then this skill aggregates their findings.

The project's issue tracker should have been provided to you, including how it tracks reviews specifically (GitHub PR/issue comments, or a local `.scratch/` log) — this determines how you document findings in the Documenting section below. If these details are missing, ask the user before proceeding.

## Coordinator contract

The coordinator is a **dispatcher**: it collects references, hands them to the two axis sub-agents, and relays their reports. All content — diff hunks, source files, standards documents, the smells catalog — is read inside a sub-agent's context, and every judgement about the code comes from a sub-agent report. The coordinator's own context holds metadata only, from its first call to the verdict block.

Dispatcher discipline, phase by phase:

- **Setup** (steps 1–3): metadata-only calls — resolve refs, verify non-emptiness via `--stat`/`stat`/`wc`, list candidate spec and standards paths. Everything a sub-agent will need travels as a path or reference in its spawn prompt.
- **Handoff** (step 4): one message, both spawns. The two axis sub-agents are the only agents in a review; the Spec prompt tells that sub-agent to locate any related code itself.
- **Waiting**: each sub-agent's completion re-invokes the coordinator automatically — **ending the turn is the wait**, and it needs no scheduling, polling, or checking. After the spawn message, end the turn: no tool calls, no status narration. When the first report arrives while the second is pending, end the turn again the same way. Each extra turn replays the coordinator's entire context; a silent wait is free.
- **Aggregation** (step 5): built from the two reports alone. The quality bar for the verdict is **faithful relay**: every sentence of the aggregate and every finding in the verdict block traces to a sub-agent report, entering exactly as reported. The reports are final — including when the axes disagree; a disagreement is presented side by side, as itself.

Two hard guardrails, kept as prohibitions because no positive phrasing covers them:

- Never open the diff artifact, a source file, a standards file, or the smells catalog in the coordinator context — not to prepare a spawn prompt, and not to check a finding before signing the verdict.
- Never run builds or tests; correctness evidence, like all evidence, comes from the sub-agent reports.

## Process

### 1. Pin the fixed point

Whatever the user said is the fixed point — a commit SHA, branch name, tag, `main`, `HEAD~5`, etc. If they didn't specify one, ask for it.

Capture the inputs once, using only the metadata calls allowed by the coordinator contract:

- Resolve the ref with `git rev-parse <fixed-point>`.
- Record the full diff command, `git diff <fixed-point>...HEAD` (three-dot, so the comparison is against the merge-base), for the sub-agents to run.
- Record the commit subjects with `git log <fixed-point>..HEAD --oneline`.
- Verify that the diff is non-empty with `git diff <fixed-point>...HEAD --stat`.

The coordinator needs the resolved ref, command, commit list, and diff stat. A bad ref or empty diff should fail here — not inside two parallel sub-agents. The sub-agents inspect the full diff.

When the invoking prompt supplies a pre-computed diff file, that file is the authoritative diff source instead of the diff command. Verify that it exists and is non-empty with `stat` or `wc`, and keep the supplied branch-point ref as review context. Both sub-agents receive its exact path and read it directly.

### 2. Identify the spec source

Look for the originating spec, in this order:

1. Issue references in the commit messages (`#123`, `Closes #45`, GitLab `!67`, etc.) — fetch via the workflow in `docs/agents/issue-tracker.md`.
2. A path the user passed as an argument.
3. A PRD/spec file under `docs/`, `specs/`, or `.scratch/` matching the branch name or feature.
4. If nothing is found, ask the user where the spec is. If they say there isn't one, pass that fact to the **Spec** sub-agent.

Identify the source with commit subjects, user-provided references, and path listings. Pass the issue/PR reference or spec path directly to the Spec sub-agent without opening it. The Spec sub-agent retrieves and reads the source. If no stable reference exists, ask the user for one; if they confirm there is no spec, pass that fact to the Spec sub-agent so it reports "no spec available".

### 3. Identify the standards sources

Use the minimum listing/search calls needed to collect paths to anything in the repo that documents how code should be written, such as `CODING_STANDARDS.md` or `CONTRIBUTING.md`. Pass those paths to the Standards sub-agent; the coordinator needs the path list, not the file contents.

On top of whatever the repo documents, the Standards axis always carries the **smell baseline** — Fowler's code smells (_Refactoring_, ch.3), cataloged in [`skills/refactoring/references/smells.md`](../refactoring/references/smells.md), applying even when a repo documents nothing. The coordinator passes this path without opening it. Two rules bind it:

- **The repo overrides.** A documented repo standard always wins; where it endorses something the baseline would flag, suppress the smell.
- **Always a judgement call.** Each smell is a labelled heuristic ("possible Feature Envy"), never a hard violation — and, like any standard here, skip anything tooling already enforces.

### 4. Spawn both sub-agents in parallel

The coordinator's setup ends at this handoff. Send a single message containing both `Agent` tool calls. Use the `general-purpose` subagent for both. Make both prompts self-sufficient by including the exact diff-file path (or, when no artifact was supplied, the full diff command), the branch-point ref, the commit list, the candidate standards-source paths, and the spec path or reference. End the turn as soon as the spawn message is sent — the Waiting phase of the coordinator contract governs everything until the reports arrive.

**Standards sub-agent prompt** — include:

- In normal mode, the full diff command and commit list. In pre-computed-diff mode, the exact diff-file path and branch-point ref instead; tell the sub-agent that the file is authoritative, must be verified non-empty, and must be read directly without invoking Git to re-derive the diff.
- The list of standards-source files you found in step 3, plus the path `skills/refactoring/references/smells.md` — tell the sub-agent to read it for the smell baseline (it has repo file access).
- The brief: "Report — per file/hunk where relevant — (a) every place the diff violates a documented standard: cite the standard (file + the rule); and (b) any baseline smell from `skills/refactoring/references/smells.md` you spot: name it and quote the hunk. Distinguish hard violations from judgement calls — documented-standard breaches can be hard, but baseline smells are always judgement calls, and a documented repo standard overrides the baseline. Skip anything tooling enforces. Under 400 words."

**Spec sub-agent prompt** — include:

- In normal mode, the diff command and commit list. In pre-computed-diff mode, the same exact diff-file path and branch-point ref used for the Standards sub-agent; tell the sub-agent to read that authoritative, non-empty file directly and not invoke Git to re-derive the diff.
- The issue/PR reference or spec path from step 2, or the user's confirmation that no spec exists. The sub-agent retrieves and reads referenced content itself.
- The instruction to locate and read any related code it needs itself; the coordinator will not locate context or verify the report.
- The brief: "Report: (a) requirements the spec asked for that are missing or partial; (b) behaviour in the diff that wasn't asked for (scope creep); (c) requirements that look implemented but where the implementation looks wrong. Quote the spec line for each finding. Under 400 words."

If the user confirmed there is no spec, still spawn the Spec sub-agent and instruct it to report "no spec available".

### 5. Aggregate

Aggregation is faithful relay of the two reports (see the coordinator contract). Present them under `## Standards` and `## Spec` headings, verbatim or lightly cleaned. Do **not** merge or rerank findings — the two axes are deliberately separate (see _Why two axes_).

End with a one-line summary: total findings per axis, and the worst issue _within each axis_ (if any). Don't pick a single winner across axes — that's the reranking the separation exists to prevent.

## Why two axes

A change can pass one axis and fail the other:

- Code that follows every standard but implements the wrong thing → **Standards pass, Spec fail.**
- Code that does exactly what the issue asked but breaks the project's conventions → **Spec pass, Standards fail.**

Reporting them separately stops one axis from masking the other.

## Documenting

**Tool-driven mode.** When the invoking prompt explicitly says that the tool records the verdict, skip the rest of this section. Do not read prior review documents or write a new review document; the verdict block is the only record. Choose this mode only from that prompt instruction, never from an inference about who invoked the skill.

If this project tracks reviews on GitHub, the verdict block you print at the
end (per the Verdict contract below) is the record — it's posted as a PR/issue
comment by the tool that invoked you. Skip the rest of this section entirely;
do not also write a `.scratch` review document.

Otherwise (local review tracking): do not write review findings into the issue
file itself — the issue file is the spec/checklist, not a review log. Instead
create a **separate review document** in the project's
`.scratch/<project>/reviews/` directory (sibling to `.scratch/<project>/issues/`),
named after the issue: `<number>-<slug>.md`, matching the issue's own filename.

Follow the format already established by existing review documents in that
directory (e.g. `reviews/12-add-optional-end-to-end-benchmark-verification.md`,
`reviews/15-extract-and-correlate-baseline-schemathesis-problems.md`):

- `# Review: <number> — <title>` heading.
- A line naming what was reviewed: the diff/commit range and files touched.
- `## Standards` and `## Spec` sections (this skill's two axes), each stating
  the problem and the suggested fix for every finding, quoting the relevant
  hunk or spec line.
- A closing `## Verdict` stating whether the issue can be closed, or what must
  be fixed first, per the Notes section below.

If a review document for this issue already exists (a prior round), add a new
`## Round N` section rather than overwriting prior rounds — see the
issue-15 review for the multi-round convention.

## Notes

If non valuable findings are found, tell the user the issue should be close and the user should move on to the next issue.

Valuable findings are:
- Spec not fully implemented or implemented wrong
- Medium or higher risk of bugs
- Codebase's standards violation and/or /refactoring smells found.
- Bad code (hard to read, hard to maintaing, etc.), like: really long functions, bad naming, etc. For a catalog, use /refactoring

## Verdict contract

Every review MUST end with exactly this block. Replace the placeholders with the review's values; do not put any text after the block:

```text
VERDICT: approve | revise
SUMMARY: <one line>
FINDINGS:
- [blocking] <file:line> — <issue>
- [nit] <file:line> — <issue>
```

`approve` means the issue can be closed. Every finding must be tagged either `[blocking]` or `[nit]`. A review with zero findings still emits the block, with an empty `FINDINGS` list and no finding lines.
