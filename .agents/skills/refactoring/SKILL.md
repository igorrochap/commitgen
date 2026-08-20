---
name: refactoring
description: Apply behavior-preserving code refactorings from Martin Fowler's "Refactoring" (2nd ed.) catalog. Use whenever the user wants to restructure existing code without changing its behavior — a general cleanup/refactor request, a named Fowler refactoring (Extract Function, Inline Variable, Introduce Parameter Object, etc.), or code flagged as smelly, hard to read, or needing tidying even without the word "refactor." Do NOT use for adding features, fixing bugs, or any change to observable behavior.
---

# Refactoring

Refactoring is the disciplined technique of restructuring existing code — altering its internal structure **without changing its observable behavior**. This skill helps you apply the refactorings cataloged in Martin Fowler's *Refactoring: Improving the Design of Existing Code* (2nd edition) as a coding agent working in a user's real codebase.

The single most important rule: **behavior must be preserved.** A refactoring that changes behavior is not a refactoring — it's a rewrite or a bug. Everything below serves that invariant.

## The core loop

Follow this loop for every refactoring the user asks for. Do not skip steps, especially the tests.

1. **Locate the target.** Confirm exactly which code the user means. If they point vaguely ("this function," "the duplication in the payment module"), read the surrounding code and name the specific target back to them before touching anything.
2. **Establish a safety net.** Find and run the existing tests covering the target. If none exist, say so and either (a) write characterization tests that pin current behavior first, or (b) ask the user how they want to proceed. Never refactor untested code silently.
3. **Identify the refactoring.** Map the user's request (or the smell you observe) to one or more named refactorings from the catalog. See `references/catalog.md`.
4. **Apply mechanically, in small steps.** Each catalog entry has a step-by-step mechanics section. Follow it. Make one small change at a time.
5. **Test after each step.** Run the tests after every meaningful step, not just at the end. If a step is small enough, the tests should still pass. If they don't, you know exactly which tiny change broke them.
6. **Commit-sized increments.** Keep each refactoring self-contained so it could be committed on its own. Never bundle a behavior change into a refactoring commit.

## Non-negotiables

- **Separate refactoring from behavior change.** Fowler's "Two Hats": you are either refactoring (structure, no behavior change) or adding function (behavior change), never both at once. If the user's request mixes them, do the refactoring first, get to green tests, then do the feature change as a distinct step. Tell the user you're doing this.
- **Small steps beat big leaps.** The whole safety of refactoring comes from small, reversible steps validated by tests. Resist the urge to do a large restructuring in one shot even when you "can see the end state."
- **Tests are the safety net, not a formality.** If you can't run the tests, flag it prominently. A refactoring you can't verify is a risk you're asking the user to accept — make that explicit rather than presenting it as done-and-safe.
- **Preserve public interfaces unless asked.** Renaming or moving something that's part of a public API can break callers you can't see. When a refactoring touches a public interface, point out the blast radius and prefer techniques that keep the old interface working (e.g., keep a delegating wrapper) unless the user confirms a breaking change is fine.

## Choosing the right refactoring

When the user names a refactoring explicitly, apply that one. When they describe a problem or a "smell," diagnose first. `references/smells.md` maps common code smells to the refactorings that address them (e.g., Long Function → Extract Function; Duplicated Code → Extract Function / Pull Up Method; Long Parameter List → Introduce Parameter Object / Preserve Whole Object).

The full catalog with mechanics is in `references/catalog.md`. Read the relevant entry before applying a refactoring you're not executing from memory — the mechanics matter, because they're specifically ordered to keep the code working at every step.

## Working with the user

- State which refactoring(s) you're applying and why, in one or two sentences, before diving in.
- If the target code lacks tests, raise it before proceeding — don't quietly refactor and hope.
- After finishing, summarize what changed structurally and confirm behavior is unchanged (and how you verified it — which tests ran, whether they passed).
- If partway through you discover the "refactoring" the user wants actually requires a behavior change, stop and clarify rather than silently altering behavior.

## Language-agnostic, mechanics-specific

The catalog is language-neutral, but mechanics adapt to the language. Use the target language's idioms (e.g., `Extract Function` produces a method in Java/Python, a free function or closure elsewhere). Respect the codebase's existing style, formatter, and linter — run them as part of your verification. Keep diffs minimal and reviewable.
