---
name: fix-review
description: Fix problems pointed out in a code review session.
---

Fix the problems pointed out in the code review.

The findings are usually given to you directly in the current conversation (e.g. a "Blocking findings" list in the prompt) — work from those first. Only look for a review document if you're explicitly pointed at one; where that document lives depends on how the project tracks reviews (a local `.scratch/<project>/reviews/` file, or a GitHub PR/issue comment) — don't assume `.scratch`.

If there's a suggested order of work, follow it. If not, resolve the most critical findings first and report what you didn't resolve and why.

/refactoring skill is your biggest friend if the findings are only about coding standards