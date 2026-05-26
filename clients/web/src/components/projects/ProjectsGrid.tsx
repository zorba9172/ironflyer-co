"use client";

// ProjectsGrid — pure responsive grid of ProjectCard tiles. State for
// filter/search/sort lives in the page (or in URL params) so this
// component stays presentation-only and can be reused by any rail that
// wants the same card grid.

import { Box } from "@mui/material";
import { ProjectCard, type ProjectCardData } from "./ProjectCard";

export interface ProjectsGridProps {
  projects: ProjectCardData[];
  onDeleted?: (id: string) => void;
}

export function ProjectsGrid({ projects, onDeleted }: ProjectsGridProps) {
  return (
    <Box
      sx={{
        display: "grid",
        gap: { xs: 1.5, md: 2 },
        gridTemplateColumns: {
          xs: "1fr",
          md: "repeat(2, minmax(0, 1fr))",
          lg: "repeat(3, minmax(0, 1fr))",
          xl: "repeat(4, minmax(0, 1fr))",
        },
        minWidth: 0,
      }}
    >
      {projects.map((p) => (
        <ProjectCard key={p.id} project={p} onDeleted={onDeleted} />
      ))}
    </Box>
  );
}
