package builder

type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type Metadata map[string]interface{}

//counterfeiter:generate -o ./fakes/directory_reader.go --fake-name PreProcessDirectoryReader . preProcessDirectoryReader
type preProcessDirectoryReader interface {
	ReadPreProcess(path string, variables map[string]interface{}) ([]Part, error)
}
