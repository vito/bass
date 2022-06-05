package proto

import "fmt"

func (ref *ThunkImageRef) Ref() (string, error) {
	repo := ref.GetRepository()
	if repo == "" {
		return "", fmt.Errorf("ref does not refer to a repository: %s", ref)
	}

	if ref.Digest != nil {
		return fmt.Sprintf("%s@%s", repo, *ref.Digest), nil
	} else if ref.Tag != nil {
		return fmt.Sprintf("%s:%s", repo, *ref.Tag), nil
	} else {
		return fmt.Sprintf("%s:latest", repo), nil
	}
}
