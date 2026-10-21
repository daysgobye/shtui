Output assets/demos/02-build-with-formulas.gif
Require go
Require ffmpeg
Require ttyd
Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 16
Set Columns 104
Set Rows 30
Set Padding 20
Set Framerate 20
Set TypingSpeed 65ms
Set Theme { "name": "Shtui Studio", "black": "#302B4A", "red": "#E06C75", "green": "#5D9F75", "yellow": "#E5B85C", "blue": "#5B6FAD", "magenta": "#C678F2", "cyan": "#55D6BE", "white": "#D9D4E2", "brightBlack": "#76718A", "brightRed": "#F28F8F", "brightGreen": "#55D6BE", "brightYellow": "#E5B85C", "brightBlue": "#7354A3", "brightMagenta": "#C678F2", "brightCyan": "#55D6BE", "brightWhite": "#F5E8C7", "background": "#1A1B26", "foreground": "#F5E8C7", "selection": "#3B3550", "cursor": "#F5E8C7" }
Env NO_COLOR ""
Env COLORTERM "truecolor"
Env TERM "xterm-256color"
Env CLICOLOR_FORCE "1"

Hide
Type "env -u NO_COLOR CLICOLOR_FORCE=1 COLORTERM=truecolor TERM=xterm-256color go run ./cmd/omasheet"
Enter
Sleep 1s
Show
Sleep 2s
Down 2
Enter
Sleep 1s
Type "ITEM"
Tab
Type "UNITS"
Tab
Type "UNIT PRICE"
Tab
Type "LINE TOTAL"
Enter
Left 3
Type "Notebook"
Tab
Type "12"
Tab
Type "4.50"
Tab
Type "=B2*C2"
Enter
Left 3
Type "Pen"
Tab
Type "25"
Tab
Type "1.20"
Tab
Type "=B3*C3"
Enter
Left 3
Type "TOTAL"
Tab
Type "=SUM(B2:B3)"
Tab
Type "=AVERAGE(C2:C3)"
Tab
Type "=SUM(D2:D3)"
Enter
Up 4
Left 3
Ctrl+P
Sleep 400ms
Type "fill color"
Enter
Sleep 300ms
Down 17
Enter
Type "#7354A3"
Enter
Sleep 300ms
Ctrl+P
Sleep 250ms
Type "toggle bold"
Enter
Down 3
Right 3
Sleep 3s
