# Label System Migration Guide

## Overview

This guide helps you migrate from the old file-based label system to the new SQLite Repository-based system introduced in Phase 9.1.

## What Changed

### Before (File-Based)

- Labels stored in `meta.yml` files
- Template content read directly from files
- No integrity checking
- No centralized label management

### After (Repository-Based)

- Labels stored in SQLite database
- Template files indexed with SHA256 hashes
- Automatic integrity validation
- Centralized CRUD operations via `deespec label` commands
- Backward compatible fallback mode

## Migration Strategies

### Strategy 1: Fresh Start (Recommended)

Best for new projects or those with few labels.

#### Step 1: Organize Templates

Create a clean template directory structure:

```bash
mkdir -p .claude
```

Move or create your template files:

```bash
# Example structure
.claude/
├── security.md
├── frontend.md
└── backend.md
```

#### Step 2: Initialize Configuration

Generate `setting.json` if not exists:

```bash
deespec init
```

Verify label configuration in `setting.json`:

```json
{
  "label_config": {
    "template_dirs": [".claude", ".deespec/prompts/labels"],
    "import": {
      "auto_prefix_from_dir": true,
      "max_line_count": 1000,
      "exclude_patterns": ["*.secret.md", "settings.*.json", "tmp/**"]
    }
  }
}
```

#### Step 3: Import Labels

```bash
# Dry run to preview
deespec label import .claude --recursive --dry-run

# Actual import
deespec label import .claude --recursive
```

#### Step 4: Update Task References

If you have existing tasks with labels in `meta.yml`, they will continue to work via fallback mode. To fully migrate:

```bash
# For each SBI with labels
deespec label attach SBI-123 security
deespec label attach SBI-123 frontend
```

### Strategy 2: Gradual Migration

Best for large projects with many existing labels.

#### Phase 1: Parallel Mode

1. Keep existing `meta.yml` files
2. Import labels to database
3. System uses database when available, falls back to files

```bash
# Import existing labels
deespec label import .deespec/prompts/labels --recursive
```

Tasks will use:
- Database labels (if registered)
- File-based labels (as fallback)

#### Phase 2: Validation

Verify both systems are synchronized:

```bash
# Check all labels
deespec label validate

# Sync any mismatches
deespec label validate --sync
```

#### Phase 3: Database-Only

Once confident, you can:
- Keep template files (recommended for git tracking)
- Remove old meta.yml label references (optional)
- System will use database exclusively

### Strategy 3: Audit and Clean

Best for projects with inconsistent label usage.

#### Step 1: Audit Current Labels

```bash
# Find all label references in meta.yml
find .deespec -name "meta.yml" -exec grep -l "labels:" {} \;

# Extract unique labels
find .deespec -name "meta.yml" -exec grep -A 10 "labels:" {} \; | \
  grep "^  - " | sort | uniq
```

#### Step 2: Consolidate Templates

Review and consolidate your template files:

```bash
# Check for duplicate content
for f in $(find .deespec/prompts/labels -name "*.md"); do
  echo "=== $f ==="
  head -5 "$f"
done
```

#### Step 3: Import Deduplicated Labels

```bash
deespec label import .deespec/prompts/labels --recursive
```

#### Step 4: Cleanup

Remove unused or duplicate labels:

```bash
# List all labels
deespec label list

# Delete unnecessary ones
deespec label delete old-label --force
```

## Verification

### Check Database Content

```bash
# List all registered labels
deespec label list

# Show details for each label
deespec label show security
deespec label show frontend
```

### Validate Integrity

```bash
# Check all labels
deespec label validate

# Expected output:
# ID  NAME       STATUS  MESSAGE
# --  ----       ------  -------
# 1   security   ✓       All files match
# 2   frontend   ✓       All files match
```

### Test SBI Workflow

Run an SBI to verify label enrichment:

```bash
# The prompt should include:
# - ## Task Labels section
# - ## Label-Specific Guidelines section
# - Full template content

deespec sbi run SBI-123
```

## Common Migration Scenarios

### Scenario 1: Hierarchical Labels

**Before:**
```
.deespec/prompts/labels/
├── perspectives/
│   ├── designer.md
│   └── developer.md
```

**After:**
```bash
deespec label import .deespec/prompts/labels/perspectives \
  --prefix-from-dir --recursive

# Creates:
# - perspectives:designer
# - perspectives:developer
```

### Scenario 2: Multiple Template Directories

**Before:**
```
.claude/
└── security.md

.deespec/prompts/labels/
└── frontend.md
```

**After:**
```bash
# Configure both directories in setting.json
{
  "label_config": {
    "template_dirs": [".claude", ".deespec/prompts/labels"]
  }
}

# Import from both
deespec label import .claude --recursive
deespec label import .deespec/prompts/labels --recursive
```

### Scenario 3: Large Template Files

**Before:**
```bash
# 1500-line template file
.claude/comprehensive-guide.md
```

**After:**
```bash
# Split into multiple files
.claude/
├── guide-part1.md (500 lines)
├── guide-part2.md (500 lines)
└── guide-part3.md (500 lines)

# Register with multiple templates
deespec label register comprehensive-guide \
  --template .claude/guide-part1.md \
  --template .claude/guide-part2.md \
  --template .claude/guide-part3.md
```

Or increase limit:
```json
{
  "label_config": {
    "import": {
      "max_line_count": 2000
    }
  }
}
```

## Backward Compatibility

The new system maintains full backward compatibility:

### Fallback Mode

If `LabelRepo` is not available or label not found in database:

```go
// Automatically falls back to file-based reading
labelPath := filepath.Join(".deespec", "prompts", "labels", labelName+".md")
content, _ := os.ReadFile(labelPath)
```

### No Breaking Changes

- Existing `meta.yml` files continue to work
- Old file-based templates are still read
- No code changes required for existing SBIs

### Migration at Your Own Pace

You can migrate:
- All at once (Strategy 1)
- Gradually (Strategy 2)
- Never (backward compatibility maintained)

## Rollback Procedure

If you need to rollback:

### Step 1: Stop Using New Commands

```bash
# Don't run:
# - deespec label register
# - deespec label import
# - deespec label validate
```

### Step 2: Keep Template Files

Ensure template files remain in place:

```bash
# Verify files exist
ls -la .claude/
ls -la .deespec/prompts/labels/
```

### Step 3: Remove Database (Optional)

```bash
# The database won't interfere, but you can remove it
rm ~/.deespec/deespec.db
```

### Step 4: Verify Fallback

```bash
# Run SBI - should use file-based labels
deespec sbi run SBI-123
```

System will automatically use file-based mode.

## Best Practices After Migration

### 1. Regular Validation

Schedule periodic integrity checks:

```bash
# Weekly or before major releases
deespec label validate --sync
```

### 2. Version Control

Commit both templates and database schema:

```bash
git add .claude/
git add .deespec/prompts/labels/
git add setting.json
git commit -m "Migrate to Repository-based label system"
```

### 3. Documentation

Document your label organization:

```markdown
# docs/labels.md

## Label Categories

### Security
- **security**: General security best practices
- **auth**: Authentication and authorization guidelines

### Architecture
- **frontend**: Frontend architecture patterns
- **backend**: Backend design guidelines
```

### 4. Team Communication

Inform team members:

```markdown
# MIGRATION_NOTICE.md

## Label System Update

We've migrated to the new SQLite-based label system.

**What you need to do:**
1. Pull latest changes: `git pull`
2. Run: `deespec init` (if setting.json doesn't exist)
3. Verify: `deespec label list`

**New commands available:**
- `deespec label import` - Batch import
- `deespec label validate` - Check integrity
- `deespec label list` - View all labels

See docs/label-system-guide.md for details.
```

## Troubleshooting

### Import Fails with "Template Not Found"

**Problem:**
```
⚠ Failed to save label security: template file not found
```

**Solution:**
```bash
# Verify file exists
ls -la .claude/security.md

# Check template_dirs configuration
cat setting.json | grep template_dirs

# Use absolute or correct relative path
deespec label register security --template .claude/security.md
```

### Duplicate Labels

**Problem:**
```
Error: label 'security' already exists (ID: 1)
```

**Solution:**
```bash
# Update existing label instead
deespec label update security --description "Updated description"

# Or force overwrite during import
deespec label import .claude --force
```

### Hash Mismatches After Migration

**Problem:**
```
⚠ Label 'security': template file has been modified since last sync
```

**Solution:**
```bash
# This is expected after migration
# Sync to update hashes
deespec label validate --sync
```

## FAQ

**Q: Do I need to migrate immediately?**
A: No, the system is fully backward compatible. Migrate when convenient.

**Q: Can I use both file-based and database labels?**
A: Yes, the system will use database labels when available and fall back to files otherwise.

**Q: What happens to my existing SBIs?**
A: They continue to work. Labels in `meta.yml` are read via fallback mode.

**Q: Can I rollback after migration?**
A: Yes, see Rollback Procedure above. Template files remain unchanged.

**Q: How do I know if migration succeeded?**
A: Run `deespec label list` and `deespec label validate` - both should succeed.

**Q: Will this affect my CI/CD?**
A: No, as long as template files are committed to git. The database is local.

## Support

For issues or questions:
- Check [Label System Guide](./label-system-guide.md)
- Review [Architecture Docs](./architecture/phase-9.1-label-system-implementation.md)
- File issue: https://github.com/YoshitsuguKoike/deespec/issues

## Next Steps

After successful migration:
1. Read [Label System Guide](./label-system-guide.md) for advanced features
2. Set up regular validation schedule
3. Document your label conventions for team
4. Consider organizing templates hierarchically
