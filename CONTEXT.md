# commitgen

A CLI that turns staged git changes into Conventional Commits messages (and branch history into pull request descriptions) using an LLM.

## Language

**Review**:
The step where a human sees a generated commit message and chooses to accept, edit, or regenerate it before the commit is created.
_Avoid_: Selection, menu, confirmation

**Agent mode**:
A run in which no human is present to perform a **Review**: the generated message is committed as-is and output is kept plain for a program to read. Entered explicitly with `--agent` or automatically when no terminal is available.
_Avoid_: Non-interactive mode, headless mode, batch mode, yes mode

**Interactive mode**:
A run with a human at a terminal, where every generated commit message goes through **Review**.
_Avoid_: TTY mode, normal mode
