---
name: measure-memory
description: "Measure local thumbnailer memory consumption while generating troubleshooting thumbnails. Use when asked for a memory assessment, memory consumption test, RSS measurement, or memory verification for thumbnail generation. Builds the service, processes all test images in runtime/originals/troubleshoot, and reports minimum and maximum RSS only."
argument-hint: "[timeout seconds]"
---

# Measure Memory

```bash
./.github/skills/measure-memory/scripts/measure.sh $ARGUMENTS
```

Run the command immediately. Report results in Kb, Mb and Gb (If applicable)
or relay errors verbatim.
