#### 6.5.1 StorageGateway インターフェース

\`\`\`go
// application/port/output/storage_gateway.go
package output

type StorageGateway interface {
    SaveArtifact(ctx context.Context, req SaveArtifactRequest) (*ArtifactMetadata, error)
    LoadArtifact(ctx context.Context, artifactID string) (*Artifact, error)
    LoadInstruction(ctx context.Context, instructionPath string) (string, error)
}
\`\`\`

#### 6.5.2 設定例

\`\`\`json
{
  "storage": {
    "type": "hybrid",
    "s3": {
      "bucket": "deespec-artifacts",
      "region": "ap-northeast-1"
    }
  }
}
\`\`\`
