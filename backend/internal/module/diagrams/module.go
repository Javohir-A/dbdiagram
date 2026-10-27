package diagrams

// Module holds every service of this domain. The app wires one and hands it to
// the HTTP layer.
type Module struct {
	Diagram *DiagramService
	Folder  *FolderService
	Version *VersionService
	Member  *MemberService
}

func New(deps Deps) *Module {
	return &Module{
		Diagram: &DiagramService{deps: deps},
		Folder:  &FolderService{deps: deps},
		Version: &VersionService{deps: deps},
		Member:  &MemberService{deps: deps},
	}
}
