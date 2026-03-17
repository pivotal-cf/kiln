package commands

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/pivotal-cf/jhanda"
)

// Carvel is a command group for Carvel/Kubernetes tile operations
type Carvel struct {
	outLogger *log.Logger
	errLogger *log.Logger
	commands  jhanda.CommandSet
	aliases   map[string]bool
	synopses  map[string]string
}

func NewCarvel(outLogger, errLogger *log.Logger) Carvel {
	c := Carvel{
		outLogger: outLogger,
		errLogger: errLogger,
		commands:  jhanda.CommandSet{},
		aliases:   map[string]bool{},
		synopses:  map[string]string{},
	}

	// Register subcommands
	c.commands["bake"] = NewCarvelBake(outLogger, errLogger)
	c.commands["upload"] = NewCarvelUpload(outLogger, errLogger)
	c.commands["publish"] = NewCarvelPublish(outLogger, errLogger)
	c.commands["re-bake"] = NewCarvelReBake(outLogger, errLogger)

	// Positional argument synopses for usage lines
	c.synopses["re-bake"] = "<bake-record>"

	// Aliases (hidden from help output)
	c.commands["rebake"] = c.commands["re-bake"]
	c.aliases["rebake"] = true
	c.synopses["rebake"] = c.synopses["re-bake"]

	return c
}

func (c Carvel) Execute(args []string) error {
	if len(args) == 0 {
		return c.printHelp()
	}

	subcommand := args[0]
	subargs := args[1:]

	if subcommand == "help" || subcommand == "-h" || subcommand == "--help" {
		if len(subargs) > 0 {
			return c.printSubcommandHelp(subargs[0])
		}
		return c.printHelp()
	}

	// Must intercept help flags here before delegating to CommandSet.Execute,
	// which would otherwise look for a nonexistent "help" command in the
	// carvel command set.
	for _, arg := range subargs {
		if arg == "-h" || arg == "--help" || arg == "help" {
			return c.printSubcommandHelp(subcommand)
		}
	}

	return c.commands.Execute(subcommand, subargs)
}

func (c Carvel) Usage() jhanda.Usage {
	var subcommandList strings.Builder
	subcommandList.WriteString("Commands for working with Carvel/Kubernetes tiles.\n\n")
	subcommandList.WriteString("Subcommands:\n")

	var names []string
	var length int
	for name := range c.commands {
		if c.aliases[name] {
			continue
		}
		names = append(names, name)
		if len(name) > length {
			length = len(name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		cmd := c.commands[name]
		paddedName := c.pad(name, " ", length)
		subcommandList.WriteString(fmt.Sprintf("  %s  %s\n", paddedName, cmd.Usage().ShortDescription))
	}
	subcommandList.WriteString("\nUse 'kiln carvel help <subcommand>' for more information about a subcommand.")

	return jhanda.Usage{
		Description:      subcommandList.String(),
		ShortDescription: "commands for Carvel/Kubernetes tiles",
		Flags:            nil,
	}
}

func (c Carvel) printHelp() error {
	var (
		length int
		names  []string
	)

	for name := range c.commands {
		if c.aliases[name] {
			continue
		}
		names = append(names, name)
		if len(name) > length {
			length = len(name)
		}
	}

	sort.Strings(names)

	fmt.Println("kiln carvel - commands for Carvel/Kubernetes tiles")
	fmt.Println()
	fmt.Println("Usage: kiln carvel <subcommand> [<args>]")
	fmt.Println()
	fmt.Println("Subcommands:")
	for _, name := range names {
		cmd := c.commands[name]
		paddedName := c.pad(name, " ", length)
		fmt.Printf("  %s  %s\n", paddedName, cmd.Usage().ShortDescription)
	}
	fmt.Println()
	fmt.Println("Use 'kiln carvel help <subcommand>' for more information about a subcommand.")

	return nil
}

func (c Carvel) printSubcommandHelp(subcommand string) error {
	cmd, ok := c.commands[subcommand]
	if !ok {
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}

	usage := cmd.Usage()
	fmt.Printf("kiln carvel %s - %s\n", subcommand, usage.ShortDescription)
	fmt.Println()
	fmt.Println(usage.Description)
	fmt.Println()

	synopsis := c.synopses[subcommand]
	if synopsis != "" {
		fmt.Printf("Usage: kiln carvel %s %s [<args>]\n", subcommand, synopsis)
	} else {
		fmt.Printf("Usage: kiln carvel %s [<args>]\n", subcommand)
	}

	if usage.Flags != nil {
		flagUsage, err := jhanda.PrintUsage(usage.Flags)
		if err != nil {
			return err
		}

		flagList := strings.Split(flagUsage, "\n")
		if len(flagList) > 0 {
			fmt.Println()
			fmt.Println("Arguments:")
			for _, flag := range flagList {
				if flag != "" {
					fmt.Printf("  %s\n", flag)
				}
			}
		}
	}

	return nil
}

func (c Carvel) pad(str, pad string, length int) string {
	for {
		str += pad
		if len(str) > length {
			return str[0:length]
		}
	}
}
