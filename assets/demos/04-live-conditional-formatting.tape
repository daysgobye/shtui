Output assets/demos/04-live-conditional-formatting.gif
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
Set TypingSpeed 55ms
Set Theme { "name": "Shtui Studio", "black": "#302B4A", "red": "#E06C75", "green": "#5D9F75", "yellow": "#E5B85C", "blue": "#5B6FAD", "magenta": "#C678F2", "cyan": "#55D6BE", "white": "#D9D4E2", "brightBlack": "#76718A", "brightRed": "#F28F8F", "brightGreen": "#55D6BE", "brightYellow": "#E5B85C", "brightBlue": "#7354A3", "brightMagenta": "#C678F2", "brightCyan": "#55D6BE", "brightWhite": "#F5E8C7", "background": "#1A1B26", "foreground": "#F5E8C7", "selection": "#3B3550", "cursor": "#F5E8C7" }
Env NO_COLOR ""
Env COLORTERM "truecolor"
Env TERM "xterm-256color"
Env CLICOLOR_FORCE "1"

Hide
Type "env -u NO_COLOR CLICOLOR_FORCE=1 COLORTERM=truecolor TERM=xterm-256color go run ./cmd/omasheet test_sheets/studio_demo.xlsx"
Enter
Sleep 2s
Show
Sleep 1s
Down 2
Right
Ctrl+P
Sleep 400ms
Type "conditional formatting"
Enter
Sleep 400ms
Type ":B8"
Tab 3
Type "60"
Tab
Enter
Sleep 300ms
Down 17
Enter
Type "#65D1B8"
Enter
Sleep 300ms
Tab 2
Enter
Tab 6
Enter
Sleep 4s
Right 6
