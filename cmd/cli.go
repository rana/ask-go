package cmd

// CLI represents the command-line interface
type CLI struct {
	Init    InitCmd    `cmd:"" help:"Initialize a new session"`
	Chat    ChatCmd    `cmd:"" default:"1" help:"Process the session (default)"`
	Cfg     CfgCmd     `cmd:"" help:"Manage configuration"`
	Version VersionCmd `cmd:"" help:"Show version information"`
}
