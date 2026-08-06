Windows gets its own title bar, drawn by Ldapper.

## One bar instead of two

Windows was putting a light title bar above a dark application. Two bars, one
of them belonging to a different design entirely, and the name repeated in
both.

The window is frameless there now. Minimise, maximise and close sit at the end
of the same bar that carries the connection — sized and spaced the way Windows
does them, so the pointer lands where it expects to, with the close button
turning coral on approach. Double-clicking the bar maximises the window, and
dragging it moves the window, as before.

macOS is untouched. Its window controls belong to the system and already sit
inside this bar looking native; drawing our own there would be a second set.

## One thing that could have gone badly

When the bar ran short of room, the item that gave way was the window controls
— which would have left no way to close the window at all. It is the
connection pill that shrinks now, and the controls never do.

## For anyone working on the interface

`npm run dev` accepts `?platform=windows`, which shows the chrome Windows gets.
It is the only way to look at it without a Windows machine, and it is how this
was checked.

## Verified

318 Go tests, 49 frontend tests — including that the three buttons exist on
Windows and that none of them are drawn on macOS — and 37 integration tests.
