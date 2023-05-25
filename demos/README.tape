Output demos/README.mp4

Set Theme "Rose Pine"
Set FontFamily "Iosevka Term"
Set Framerate 24
Set FontSize 24
Set Padding 16
Set Width 1920
Set Height 1080

Type "bass demos/booklit/build.bass > booklit.json"
Sleep 500ms
Enter
MatchLine "^>"
Sleep 500ms

Type "bass --export < booklit.json"
Sleep 500ms
Enter
MatchLine "^>"
Sleep 500ms

Type "bass --export < booklit.json | tar -xf -"
Sleep 500ms
Enter
MatchLine "^>"
Sleep 500ms

Type "./booklit_linux_amd64 --version"
Sleep 500ms
Enter
MatchLine "^>"
Sleep 5s
