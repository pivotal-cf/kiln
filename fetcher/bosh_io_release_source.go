package fetcher

import "github.com/pivotal-cf/kiln/internal/cargo"

type BOSHIOReleaseSource struct{

}


func (r BOSHIOReleaseSource) GetMatchedRelease(assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error) {
	return nil, nil, nil
}