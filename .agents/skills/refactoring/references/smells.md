# Code Smells → Refactorings

When the user describes a *problem* rather than naming a refactoring, diagnose the smell and pick from these. These are Fowler's "Bad Smells in Code." Multiple refactorings often apply — pick the smallest one that addresses the smell, apply it, re-evaluate.

| Smell | What it looks like | Candidate refactorings |
|---|---|---|
| **Mysterious Name** | A name doesn't say what the thing is/does | Rename Variable, Change Function Declaration, Rename Field |
| **Duplicated Code** | Same structure in more than one place | Extract Function; Slide Statements then extract; Pull Up Method (if in sibling subclasses) |
| **Long Function** | A function that's hard to hold in your head | Extract Function; Replace Temp with Query; Introduce Parameter Object; Decompose Conditional; Replace Conditional with Polymorphism |
| **Long Parameter List** | Too many parameters | Replace Parameter with Query; Preserve Whole Object; Introduce Parameter Object; Remove Flag Argument |
| **Global / Mutable Data** | Widely accessible mutable state | Encapsulate Variable |
| **Divergent Change** | One module changed for many different reasons | Split Phase; Move Function; Extract Function/Class |
| **Shotgun Surgery** | One change forces edits across many modules | Move Function/Field to bring things together; Combine Functions into Class |
| **Feature Envy** | A function more interested in another module's data | Move Function; Extract Function then move |
| **Data Clumps** | Same group of data items appearing together | Introduce Parameter Object; Preserve Whole Object; Extract Class |
| **Primitive Obsession** | Primitives used where a small type would be clearer | Replace Primitive with Object; Replace Type Code with Subclasses; Introduce Parameter Object |
| **Repeated Switches / Type-code conditionals** | Same `switch`/`if` on a type in many places | Replace Conditional with Polymorphism; Replace Type Code with Subclasses |
| **Loops** | Raw loops obscuring intent | Replace Loop with Pipeline (map/filter/reduce) |
| **Nested Conditionals** | Deep `if`/`else` nesting | Replace Nested Conditional with Guard Clauses; Decompose Conditional |
| **Lazy Element** | A class/function not pulling its weight | Inline Function; Inline Class; Collapse Hierarchy |
| **Speculative Generality** | Machinery built for needs that never came | Collapse Hierarchy; Inline Function/Class; Remove Dead Code; Change Function Declaration (drop unused params) |
| **Temporary Field** | A field only set/used in certain circumstances | Extract Class; Introduce Special Case |
| **Message Chains** | `a.getB().getC().getD()` | Hide Delegate; Extract Function then Move Function |
| **Middle Man** | A class that only delegates | Remove Middle Man; Inline Function |
| **Insider Trading** | Modules whispering to each other too much | Move Function/Field; Hide Delegate; Replace Subclass/Superclass with Delegate |
| **Large Class** | A class doing too much | Extract Class; Extract Superclass; Replace Type Code with Subclasses |
| **Alternative Classes with Different Interfaces** | Similar classes, mismatched APIs | Change Function Declaration; Move Function; Extract Superclass |
| **Data Class** | A class that's only fields + accessors | Move Function (move behavior in); Encapsulate Record; Remove Setting Method |
| **Refused Bequest** | Subclass ignores inherited members | Push Down Method/Field; Replace Subclass/Superclass with Delegate |
| **Comments (as deodorant)** | Comments explaining bad code | Extract Function (name it after the comment); Rename; Introduce Assertion |

## Diagnostic approach

1. Read the code the user pointed at, plus enough surrounding context to understand callers and data flow.
2. Name the dominant smell(s). Don't over-diagnose — pick the one causing the most reading pain.
3. Choose the smallest refactoring that relieves it. Apply it. Re-read. The right next step is often obvious only after the first one lands.
4. Chain refactorings deliberately: e.g., a Long Function often becomes tractable after Extract Variable → Replace Temp with Query → Extract Function, in that order. Test between each.
