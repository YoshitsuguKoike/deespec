# Label System User Guide

## Overview

The Label System allows you to organize and enhance your SBI/PBI/EPIC tasks with reusable template-based guidelines. Labels are stored in SQLite and provide AI context enrichment through template files.

## Quick Start

### 1. Initialize Configuration

First, generate default configuration:

```bash
deespec init
```

This creates `setting.json` with label configuration:

```json
{
  "label_config": {
    "template_dirs": [".claude", ".deespec/prompts/labels"],
    "import": {
      "auto_prefix_from_dir": true,
      "max_line_count": 1000,
      "exclude_patterns": ["*.secret.md", "settings.*.json", "tmp/**"]
    },
    "validation": {
      "auto_sync_on_mismatch": false,
      "warn_on_large_files": true
    }
  }
}
```

### 2. Create Template Files

Create label template files in `.claude/` or `.deespec/prompts/labels/`:

```bash
mkdir -p .claude
cat > .claude/security.md <<EOF
# Security Guidelines

When implementing this task, ensure:

1. **Input Validation**
   - Validate all user inputs
   - Sanitize data before processing
   - Use parameterized queries for database operations

2. **Authentication & Authorization**
   - Implement proper authentication checks
   - Verify user permissions before operations
   - Use secure session management

3. **Error Handling**
   - Don't expose sensitive information in errors
   - Log security events appropriately
   - Handle edge cases gracefully
EOF
```

### 3. Import Labels

Import labels from your template directory:

```bash
# Import all .md files from .claude directory
deespec label import .claude --recursive

# Dry run to preview what will be imported
deespec label import .claude --recursive --dry-run

# Import with directory structure as label prefix
deespec label import .claude/perspectives --prefix-from-dir
```

Output:
```
Found 3 template files

  ✓ Imported: security (template: .claude/security.md, 25 lines)
  ✓ Imported: frontend (template: .claude/frontend.md, 18 lines)
  ✓ Imported: backend (template: .claude/backend.md, 30 lines)

Summary: 3 imported, 0 skipped
```

## Core Commands

### Register a Label

Create a new label with template:

```bash
# Register with single template
deespec label register security \
  --description "Security best practices" \
  --template .claude/security.md \
  --priority 10

# Register with multiple templates
deespec label register full-stack \
  --description "Full-stack development" \
  --template .claude/frontend.md \
  --template .claude/backend.md \
  --priority 5
```

### List Labels

View all registered labels:

```bash
# List active labels
deespec label list

# List all labels (including inactive)
deespec label list --all

# JSON output
deespec label list --json
```

Output:
```
ID  NAME       DESCRIPTION                 TEMPLATES  PRIORITY  ACTIVE
--  ----       -----------                 ---------  --------  ------
1   security   Security best practices     1 files    10        ✓
2   frontend   Frontend architecture       1 files    5         ✓
3   backend    Backend patterns            1 files    5         ✓

Total: 3 labels
```

### Show Label Details

Display detailed information about a label:

```bash
# By name
deespec label show security

# By ID
deespec label show 1
```

Output:
```
Label: security (ID: 1)
Description: Security best practices
Priority: 10
Active: true
Line Count: 25
Last Synced: 2025-10-09 14:30:00

Templates:
  - .claude/security.md (hash: a1b2c3d4...)

```

### Update Label

Modify label properties:

```bash
# Update description
deespec label update security --description "Updated security guidelines"

# Update priority
deespec label update security --priority 15

# Deactivate label
deespec label update security --deactivate

# Reactivate label
deespec label update security --activate
```

### Attach Labels to Tasks

Associate labels with tasks:

```bash
# Attach label to SBI
deespec label attach SBI-123 security

# Attach with display position
deespec label attach SBI-123 frontend --position 1
```

### Detach Labels

Remove label association:

```bash
deespec label detach SBI-123 security
```

### Validate Label Integrity

Check if template files have been modified:

```bash
# Validate all labels
deespec label validate

# Validate specific label
deespec label validate security

# Auto-sync modified labels
deespec label validate --sync

# Show detailed hash information
deespec label validate --details
```

Output:
```
Validating 3 label(s)...

ID  NAME       STATUS  MESSAGE
--  ----       ------  -------
1   security   ⚠       File content changed
2   frontend   ✓       All files match
3   backend    ✓       All files match

Summary:
  ✓ OK:       2
  ⚠ Modified: 1

Tip: Use --sync to automatically update hashes for modified files.
```

### Delete Label

Remove a label:

```bash
# Delete with confirmation
deespec label delete security

# Force delete without confirmation
deespec label delete security --force
```

## Advanced Usage

### Hierarchical Labels

Organize labels using directory structure:

```bash
# Create hierarchical structure
mkdir -p .claude/perspectives
cat > .claude/perspectives/designer.md <<EOF
# Designer Perspective
Focus on user experience and visual design...
EOF

# Import with prefix
deespec label import .claude/perspectives --prefix-from-dir
```

This creates label: `perspectives:designer`

### Exclusion Patterns

Configure exclusion patterns in `setting.json`:

```json
{
  "label_config": {
    "import": {
      "exclude_patterns": [
        "*.secret.md",
        "*.draft.md",
        "tmp/**",
        "archive/**"
      ]
    }
  }
}
```

### Multiple Template Directories

Configure multiple search paths:

```json
{
  "label_config": {
    "template_dirs": [
      ".claude",
      ".deespec/prompts/labels",
      "docs/templates"
    ]
  }
}
```

Files are resolved in order of directory priority.

### Line Count Limit

Templates exceeding the limit are rejected during import:

```json
{
  "label_config": {
    "import": {
      "max_line_count": 1000
    }
  }
}
```

## Integration with SBI Workflow

### How Labels Enrich AI Prompts

When you run `deespec sbi run`, labels attached to the SBI are automatically included in the AI prompt:

```markdown
## Task Labels
This task is tagged with: security, frontend

## Label-Specific Guidelines
The following guidelines apply based on the task labels:

### Label: security
# Security Guidelines
...

### Label: frontend
# Frontend Architecture
...
```

### File Integrity Warnings

If template files have been modified since last sync, warnings appear in the prompt:

```markdown
## Label Warnings
⚠ Label 'security': template file has been modified since last sync
```

This ensures the AI uses the latest template content while notifying you of changes.

### Backward Compatibility

The system gracefully falls back to file-based mode if:
- LabelRepository is not available
- Label is not found in database

This ensures existing workflows continue to work.

## Best Practices

### 1. Organize Templates by Category

```
.claude/
├── perspectives/
│   ├── designer.md
│   ├── developer.md
│   └── reviewer.md
├── domains/
│   ├── security.md
│   ├── performance.md
│   └── accessibility.md
└── workflows/
    ├── tdd.md
    └── review-process.md
```

### 2. Keep Templates Focused

Each template should focus on a single concern:
- ✅ Good: `security.md` - Security guidelines
- ❌ Bad: `everything.md` - All guidelines in one file

### 3. Use Priorities Wisely

Higher priority labels appear first in AI prompts:
- Critical guidelines: 10-15
- Standard guidelines: 5-9
- Optional context: 1-4

### 4. Regular Validation

Run validation regularly to detect template drift:

```bash
# Weekly validation
deespec label validate --sync
```

### 5. Version Control Templates

Commit template files to git:

```bash
git add .claude/
git commit -m "Add security and frontend label templates"
```

### 6. Document Label Purpose

Use clear descriptions:

```bash
deespec label register security \
  --description "Security best practices for authentication, input validation, and error handling"
```

## Troubleshooting

### Label Not Found During Import

**Problem:** `⚠ Label 'security' not found in database (using fallback)`

**Solution:** Import the label first:
```bash
deespec label import .claude --recursive
```

### Template File Missing

**Problem:** `❌ Label 'security': template file not found`

**Solution:** Restore the template file or update the label:
```bash
# Restore file
git checkout .claude/security.md

# Or update label to point to new location
deespec label update security --template .claude/new-security.md
```

### Hash Mismatch

**Problem:** `⚠ Label 'security': template file has been modified since last sync`

**Solution:** Sync the label to update hash:
```bash
deespec label validate security --sync
```

### Import Skipped - Line Count Exceeded

**Problem:** `⊗ Skipping security.md: exceeds max line count (1200 > 1000)`

**Solution:** Either split the template or increase limit in `setting.json`:
```json
{
  "label_config": {
    "import": {
      "max_line_count": 1500
    }
  }
}
```

## Migration from File-Based Labels

See [Migration Guide](./label-system-migration-guide.md) for detailed instructions on migrating from the old file-based label system to the new Repository-based system.

## API Reference

### Label Model Fields

- `id`: Unique identifier (auto-generated)
- `name`: Label name (unique, lowercase with hyphens)
- `description`: Human-readable description
- `template_paths`: Array of template file paths
- `content_hashes`: SHA256 hashes of template files
- `line_count`: Total lines across all templates
- `priority`: Merge priority (higher = earlier in prompt)
- `is_active`: Active status flag
- `parent_label_id`: Optional hierarchical parent
- `color`: Optional UI color
- `last_synced_at`: Last file sync timestamp

### Configuration Fields

See [setting.json Schema](./setting-schema.md) for complete configuration reference.

## See Also

- [Migration Guide](./label-system-migration-guide.md)
- [Architecture Documentation](./architecture/phase-9.1-label-system-implementation.md)
- [CHANGELOG](../CHANGELOG.md)
