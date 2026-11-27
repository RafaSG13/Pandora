# Run scripts/screenshots/record.sh instead of calling vhs directly.
Output assets/demo-cli.gif
Require pandora-cli-demo

Set Shell "bash"
Set FontSize 17
Set Width 1400
Set Height 600
Set Padding 24
Set WindowBar Colorful
Set BorderRadius 10
Set TypingSpeed 35ms
Set Theme {"name":"Pandora","black":"#1A1325","red":"#F87171","green":"#10B981","yellow":"#FBBF24","blue":"#3B82F6","magenta":"#A855F7","cyan":"#22D3EE","white":"#E2E8F0","brightBlack":"#64748B","brightRed":"#FCA5A5","brightGreen":"#34D399","brightYellow":"#FACC15","brightBlue":"#60A5FA","brightMagenta":"#C084FC","brightCyan":"#67E8F9","brightWhite":"#F8FAFC","background":"#130E1C","foreground":"#F8FAFC","selection":"#2E1F4D","cursor":"#A855F7"}

Hide
Type "unset NO_COLOR CI; export TERM=xterm-256color COLORTERM=truecolor LANG=en_US.UTF-8; clear"
Enter
Type "pandora-cli-demo"
Enter
Sleep 1s
Type "clear"
Enter
Sleep 500ms
Show

Sleep 500ms
Type "pandora list"
Enter
Sleep 2s

Type `pandora run -s stripe-api-key=STRIPE_KEY -- sh -c 'echo "STRIPE_KEY has ${#STRIPE_KEY} chars"'`
Enter
Sleep 2s
Type `echo "afterwards: [$STRIPE_KEY]"`
Enter
Sleep 2500ms

Type "clear"
Enter
Sleep 500ms
Type "pandora id > laptop.id"
Enter
Sleep 1s
Type "scp laptop.id server:"
Enter
Sleep 1s
Type "ssh server"
Enter
Sleep 800ms
Type "pandora peer add laptop laptop.id"
Enter
Sleep 2s
Type "y"
Enter
Sleep 1s
Type "pandora id > server.id"
Enter
Sleep 1s
Type "scp server.id laptop:"
Enter
Sleep 1s
Type "exit"
Enter
Sleep 600ms

Type "clear"
Enter
Sleep 500ms
Type "pandora peer add server server.id"
Enter
Sleep 2s
Type "y"
Enter
Sleep 1s
Type "pandora export --values --to server -s stripe-api-key -o share.pandora"
Enter
Sleep 1500ms
Type "scp share.pandora server:"
Enter
Sleep 1s
Type "ssh server"
Enter
Sleep 800ms
Type "pandora import share.pandora --values"
Enter
Sleep 2s
Type "pandora list"
Enter
Sleep 3s

Hide
Type "exit"
Enter
Type "exit"
Enter
Sleep 500ms
