package baking

type InstanceGroupsService struct {
	logger logger
	reader directoryReader
}

func NewInstanceGroupsService(logger logger, reader directoryReader) InstanceGroupsService {
	return InstanceGroupsService{
		logger: logger,
		reader: reader,
	}
}

func (igs InstanceGroupsService) FromDirectories(directories []string) (map[string]interface{}, error) {
	var instanceGroups map[string]interface{}
	if directories != nil {
		igs.logger.Println("Reading instance group files...")
		instanceGroups = map[string]interface{}{}
		for _, instanceGroupDir := range directories {
			instanceGroupsInDirectory, err := igs.reader.Read(instanceGroupDir)
			if err != nil {
				return nil, err
			}

			for _, instanceGroup := range instanceGroupsInDirectory {
				instanceGroups[instanceGroup.Name] = instanceGroup.Metadata
			}
		}
	}

	return instanceGroups, nil
}
