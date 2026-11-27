# Run scripts/screenshots/record.sh instead of calling vhs directly.
Output assets/demo.gif
Require pandora-demo

Set Shell "bash"
Set FontSize 15
Set Width 1400
Set Height 820
Set Padding 24
Set WindowBar Colorful
Set BorderRadius 10
Set TypingSpeed 70ms
Set Theme {"name":"Pandora","black":"#1A1325","red":"#F87171","green":"#10B981","yellow":"#FBBF24","blue":"#3B82F6","magenta":"#A855F7","cyan":"#22D3EE","white":"#E2E8F0","brightBlack":"#64748B","brightRed":"#FCA5A5","brightGreen":"#34D399","brightYellow":"#FACC15","brightBlue":"#60A5FA","brightMagenta":"#C084FC","brightCyan":"#67E8F9","brightWhite":"#F8FAFC","background":"#130E1C","foreground":"#F8FAFC","selection":"#2E1F4D","cursor":"#A855F7"}

Hide
Type "unset NO_COLOR CI; export TERM=xterm-256color COLORTERM=truecolor LANG=en_US.UTF-8; clear"
Enter
Type "pandora-demo"
Enter
Sleep 3s
Show

Sleep 2s
Screenshot assets/screenshots/browse.png

Down 2
Sleep 600ms
Type "v"
Sleep 1.5s
Screenshot assets/screenshots/reveal.png
Sleep 1s

Type "/"
Sleep 400ms
Type "tag:prod"
Sleep 2s
Escape
Sleep 800ms

Type "f"
Sleep 1.5s
Screenshot assets/screenshots/picker.png
Escape
Sleep 800ms

Type "t"
Sleep 1.5s
Screenshot assets/screenshots/compact.png
Type "t"
Sleep 800ms

Type "n"
Sleep 800ms
Type "cloudflare-dns"
Tab
Type "Infra"
Tab
Type "ops@example.com"
Tab
Ctrl+G
Sleep 2s
Screenshot assets/screenshots/form.png
Escape
Sleep 500ms
Escape
Sleep 1s

Type "?"
Sleep 2s
Screenshot assets/screenshots/help.png
Escape
Sleep 1s

Hide
Type "q"
Sleep 500ms
