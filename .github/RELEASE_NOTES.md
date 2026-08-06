Which build you are running is now on screen, and the README shows what
Ldapper looks like.

## The version, where you will look for it

Bottom right of the status bar. It is the one place always on screen, it is
the first thing a bug report needs, and a separate About window for a tool
this size is a click nobody wants.

A build made without the stamp says `dev` rather than claiming a release it is
not, and a build from a working tree with uncommitted changes gets a `+`. The
tooltip carries the commit.

## Screenshots

The README now shows browsing, searching, the filter library and the
connection dialog.

## Two more fixes, found while taking them

- The heading above the attribute table showed `Volkova\2C Anna`. The
  unescaping added for the tree had never been applied there, so the two panes
  disagreed about the same name.
- Double-clicking a tree row to expand it also selected the label and started
  a drag, which made the row look like it had come loose.
- The scope selector wrapped "One level" onto two lines, making that segment
  taller than the ones beside it.

## For anyone working on the interface

`npm run dev` in `frontend/` now runs the interface against a stand-in for the
Go side, so it can be worked on without building the application. It is the
same components and the same stylesheets — it is how the screenshots above
were taken.

## Verified

318 Go tests, 46 frontend tests, 37 integration tests.
