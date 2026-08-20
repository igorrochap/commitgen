# Refactoring Catalog

Named refactorings from Martin Fowler's *Refactoring* (2nd ed.). Each entry gives the intent, when to reach for it, and the ordered mechanics that keep code working at every step. Follow the mechanics in order and run tests between steps.

## Contents

- [Composing Methods](#composing-methods): Extract Function, Inline Function, Extract Variable, Inline Variable, Change Function Declaration, Replace Temp with Query, Split Variable
- [Moving Features](#moving-features): Move Function, Move Field, Move Statements into Function, Slide Statements
- [Organizing Data](#organizing-data): Split Variable, Rename Variable, Replace Magic Literal, Encapsulate Variable, Encapsulate Record/Collection
- [Simplifying Conditionals](#simplifying-conditional-logic): Decompose Conditional, Consolidate Conditional, Replace Nested Conditional with Guard Clauses, Replace Conditional with Polymorphism, Introduce Special Case, Introduce Assertion
- [Refactoring APIs](#refactoring-apis): Separate Query from Modifier, Parameterize Function, Remove Flag Argument, Preserve Whole Object, Replace Parameter with Query, Introduce Parameter Object
- [Dealing with Inheritance](#dealing-with-inheritance): Pull Up Method/Field, Push Down Method/Field, Extract Superclass, Replace Subclass with Delegate, Replace Superclass with Delegate

---

## Composing Methods

### Extract Function
**Intent:** Turn a fragment of code that can be grouped together into its own named function.
**When:** A function is too long; a comment explains what a block does; a block is used more than once; code operates at mixed levels of abstraction.
**Mechanics:**
1. Create a new function named after *what it does*, not how.
2. Copy the extracted code from source into the new function.
3. Identify variables referenced in the fragment that are local to the source. Pass ones read as parameters; a single variable assigned and used later becomes the return value. If multiple are assigned, reconsider (extract a smaller piece, or return an object).
4. Replace the extracted fragment in the source with a call to the new function.
5. Test.

### Inline Function
**Intent:** Replace a call with the function's body when the body is as clear as the name.
**When:** Indirection adds no value; a function's body is just as readable as its name; a group of badly factored functions will be reorganized.
**Mechanics:**
1. Check the function isn't polymorphic (don't inline a method overridden in subclasses).
2. Find all callers.
3. Replace each call with the function's body.
4. Test after each replacement.
5. Remove the function.

### Extract Variable
**Intent:** Give a name to a complex expression via a local variable.
**When:** An expression is hard to read; a subexpression is worth naming for clarity.
**Mechanics:**
1. Ensure the expression has no side effects.
2. Declare an immutable variable, set it to the expression.
3. Replace the original expression with the variable.
4. Test.

### Inline Variable
**Intent:** Replace a variable that's just a name for an expression with the expression itself.
**When:** The variable's name doesn't communicate more than the expression, and it gets in the way of other refactorings.
**Mechanics:** Check the RHS is side-effect-free; replace references with the expression one at a time, testing; remove the declaration.

### Change Function Declaration (a.k.a. Rename Function / Change Signature)
**Intent:** Change a function's name or parameters.
**When:** A name doesn't reveal intent; a parameter should be added, removed, or reordered.
**Mechanics (simple):** Change the declaration, update all callers, test.
**Mechanics (migration, for published/widely-used functions):**
1. Refactor the body if needed so the new declaration is easy to build.
2. Create a new function with the new declaration; make the old one call it.
3. Test.
4. Migrate callers to the new function one at a time, testing.
5. Remove the old function once no callers remain.

### Replace Temp with Query
**Intent:** Replace a temporary variable holding an expression result with a function.
**When:** A temp is assigned once from an expression, and extracting it as a query enables further extraction/deduplication.
**Mechanics:** Ensure the temp is assigned once and the expression has no side effects; extract the RHS into a query function; replace the temp with calls to the query; test.

### Split Variable
**Intent:** Give each responsibility of a reused variable its own variable.
**When:** A variable is assigned more than once but isn't a loop variable or a legitimate accumulator — it's being reused for two different purposes.
**Mechanics:** Rename at the first declaration and its uses up to the second assignment; declare a fresh variable at the second assignment; test; repeat.

---

## Moving Features

### Move Function
**Intent:** Move a function to the context (module/class) it belongs with.
**When:** A function references elements of another context more than its own; better homes exist for it.
**Mechanics:** Examine what the function uses in its current scope; check for callers; copy to the target context and adjust to fit; turn the source function into a delegating call or replace all callers; test; remove the original if fully migrated.

### Move Field
**Intent:** Move a data field to the record/class where it's more relevant.
**Mechanics:** Encapsulate the field if not already; create the field (and accessors) in the target; adjust source accessors to use the target; test; remove the source field once callers use the target.

### Slide Statements
**Intent:** Move related code together so it can be understood or extracted as a unit.
**When:** Declarations and their uses are scattered; you want to group before Extract Function.
**Mechanics:** Identify the target position; check the fragment being moved doesn't depend on / isn't depended upon by intervening code (no interfering references or side effects); move it; test.

### Move Statements into Function
**Intent:** Move repeated statements that always accompany a function call into the function itself.
**Mechanics:** If callers are scattered, use Slide Statements to place the repeated code adjacent to the call; move the statements into the callee; test.

---

## Organizing Data

### Rename Variable / Rename Field
**Intent:** A clear name is the cheapest documentation. Rename to reveal intent.
**Mechanics:** If widely used, consider Encapsulate Variable first. Rename the declaration and all references; test. Rely on the language's rename tooling where available, but verify string-based references (reflection, serialization keys).

### Replace Magic Literal
**Intent:** Replace a bare literal with a named constant.
**Mechanics:** Declare a constant; find each occurrence of the literal that means the same thing (not coincidental matches!); replace with the constant; test.

### Encapsulate Variable / Encapsulate Field
**Intent:** Route access to data through functions so you can later change how it's stored, add validation, or move it.
**When:** Data is widely accessed directly and you need a chokepoint before further changes.
**Mechanics:** Create getter/setter (or accessor functions); replace direct references with calls one at a time, testing; restrict direct visibility of the variable.

### Encapsulate Collection
**Intent:** Prevent callers from mutating a collection field directly.
**Mechanics:** Encapsulate the collection field; provide add/remove methods; make the getter return a copy or read-only view; update callers; test.

---

## Simplifying Conditional Logic

### Decompose Conditional
**Intent:** Extract the condition, then-branch, and else-branch into well-named functions.
**When:** A complex conditional obscures *why* something happens.
**Mechanics:** Apply Extract Function to the condition and to each leg; test after each extraction.

### Consolidate Conditional Expression
**Intent:** Combine a sequence of conditionals that have the same result into one, then extract it.
**When:** Several checks lead to identical actions.
**Mechanics:** Ensure the conditionals are side-effect-free; combine with `&&`/`||` as appropriate; test; consider Extract Function on the combined condition.

### Replace Nested Conditional with Guard Clauses
**Intent:** Flatten nested `if`/`else` by handling special/exit cases first and returning early.
**When:** Nesting hides the normal path; exceptional cases are entangled with the main logic.
**Mechanics:** For each check that is an exit condition, replace it with a guard clause that returns early; test after each; simplify remaining logic.

### Replace Conditional with Polymorphism
**Intent:** Move each branch of a conditional (especially a type-switch) into an overriding method of a subclass/variant.
**When:** The same conditional on a type code appears in multiple places; behavior varies by type.
**Mechanics:** Ensure a class structure exists (create it — possibly via Replace Type Code with Subclasses — if needed); move the conditional into a superclass method; for each leg, override in the appropriate subclass and remove that leg from the superclass; leave any default in the superclass; test after each move.

### Introduce Special Case (Null Object)
**Intent:** Replace repeated checks for a special value (often null) with a special-case object that provides default behavior.
**Mechanics:** Add a check method to the subject; create the special-case object with the common default behavior; replace the repeated checks with use of the special case; test.

### Introduce Assertion
**Intent:** Make an assumed condition explicit with an assertion.
**When:** A section of code works only if some condition is true, and that assumption is implicit.
**Mechanics:** Add an assertion for the assumption. Assertions must not change behavior — code must run identically with them removed.

---

## Refactoring APIs

### Separate Query from Modifier
**Intent:** Split a function that both returns a value and has side effects into two functions.
**When:** A function returns a value *and* changes state — this couples callers to the side effect.
**Mechanics:** Copy the function, name it as a pure query, remove side effects from it; find callers, have them call the query then the original (temporarily); remove the return value from the modifier; test.

### Parameterize Function
**Intent:** Merge functions that differ only by literal values into one function taking a parameter.
**Mechanics:** Pick one function; add parameters for the values that differ; replace the literal with the parameter; redirect other callers and delete the redundant functions; test.

### Remove Flag Argument
**Intent:** Replace a boolean/enum flag that selects behavior with explicit, separately named functions.
**When:** A caller passes a literal `true`/`false` and can't tell what it means.
**Mechanics:** Create an explicit function for each value of the flag; redirect callers to the explicit functions; test.

### Preserve Whole Object
**Intent:** Pass the whole object instead of several values pulled from it.
**When:** Several parameters are all derived from one object.
**Mechanics:** Add the whole object as a parameter; replace uses of the individual values with accessing them off the object; remove the now-unused parameters; test.

### Introduce Parameter Object
**Intent:** Group a recurring clump of parameters into a single object.
**When:** The same group of data items travels together through many signatures (a "data clump").
**Mechanics:** Create a class/record for the group; add it as a parameter; move the group's members onto it; replace each old parameter's usage with access via the new object; remove old parameters; test.

---

## Dealing with Inheritance

### Pull Up Method / Pull Up Field
**Intent:** Move identical (or unifiable) methods/fields from subclasses into the superclass to remove duplication.
**Mechanics:** Confirm the members are truly identical (unify signatures/bodies first if not — e.g., via Extract Function); move one member to the superclass; delete the subclass copies one at a time; test after each.

### Push Down Method / Push Down Field
**Intent:** Move a member used by only some subclasses down to those subclasses.
**Mechanics:** Copy the member into each subclass that needs it; remove from the superclass; test.

### Extract Superclass
**Intent:** Create a superclass to hold common features of classes with similar data/behavior.
**Mechanics:** Create an empty superclass, make the originals extend it; Pull Up common fields/methods one at a time, testing after each; examine remaining differences.

### Replace Subclass with Delegate / Replace Superclass with Delegate
**Intent:** Favor composition over inheritance when inheritance is causing problems (e.g., single-inheritance limits, or the relationship isn't truly "is-a").
**Mechanics:** These are larger moves — read the full Fowler mechanics carefully. In short: create the delegate, route the varying behavior through it, and remove the inheritance relationship, keeping tests green at each step.

---

## When you don't find an exact match

The catalog isn't exhaustive here — it covers the most-used refactorings. If the user names a refactoring not listed (e.g., *Replace Type Code with Subclasses*, *Split Phase*, *Combine Functions into Class/Transform*, *Substitute Algorithm*, *Replace Loop with Pipeline*, *Remove Dead Code*), apply the general principle: **small behavior-preserving steps, tested after each.** Recall Fowler's mechanics for it if you know them, and if unsure, tell the user your planned steps before executing so they can course-correct.
