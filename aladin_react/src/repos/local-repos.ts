import { createArtifactRowRepo, type ArtifactRowRepo } from "@/repos/artifacts/artifact-row-repo";
import { createBrowserNodeRepo, type BrowserNodeRepo } from "@/repos/workspace/browser-node-repo";
import {
  createLocalReposAdmin,
  type LocalReposAdmin,
} from "@/repos/local-repos-admin";
import { createPageMetadataRepo, type PageMetadataRepo } from "@/repos/pages/page-metadata-repo";

export interface LocalRepos {
  browser: BrowserNodeRepo;
  artifacts: ArtifactRowRepo;
  pages: PageMetadataRepo;
}

export interface LocalReposBundle {
  repos: LocalRepos;
  admin: LocalReposAdmin;
}

export function createLocalRepos(): LocalReposBundle {
  return {
    repos: {
      browser: createBrowserNodeRepo(),
      artifacts: createArtifactRowRepo(),
      pages: createPageMetadataRepo(),
    },
    admin: createLocalReposAdmin(),
  };
}
