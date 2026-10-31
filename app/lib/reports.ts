import fs from "fs";
import path from "path";
import matter from "gray-matter";

export interface ReportAuthor {
  name: string;
  role?: string;
  avatar?: string;
}

export interface ReportMeta {
  slug: string;
  title: string;
  issue: number;
  date: string;
  readTime: string;
  excerpt: string;
  author: ReportAuthor;
  tags: string[];
  highlights?: string[];
  featured?: boolean;
}

function getContentDirectory(): string {
  // Check process.cwd()/content/reports, then process.cwd()/app/content/reports
  const candidates = [
    path.join(process.cwd(), "content/reports"),
    path.join(process.cwd(), "app/content/reports"),
  ];

  for (const dir of candidates) {
    if (fs.existsSync(dir)) {
      return dir;
    }
  }

  return candidates[0];
}

export function getAllReports(): ReportMeta[] {
  const contentDir = getContentDirectory();

  if (!fs.existsSync(contentDir)) {
    return [];
  }

  const files = fs.readdirSync(contentDir).filter((file) => file.endsWith(".mdx") || file.endsWith(".md"));

  const reports: ReportMeta[] = files.map((fileName) => {
    const slug = fileName.replace(/\.mdx?$/, "");
    const fullPath = path.join(contentDir, fileName);
    const fileContents = fs.readFileSync(fullPath, "utf-8");
    const { data } = matter(fileContents);

    return {
      slug,
      title: data.title || "Untitled Issue",
      issue: Number(data.issue) || 0,
      date: data.date || "",
      readTime: data.readTime || "5 min read",
      excerpt: data.excerpt || "",
      author: data.author || {
        name: "Auvis Core Team",
        role: "Engineering Dispatch",
        avatar: "/auvis-final.svg",
      },
      tags: Array.isArray(data.tags) ? data.tags : [],
      highlights: Array.isArray(data.highlights) ? data.highlights : [],
      featured: Boolean(data.featured),
    };
  });

  // Sort descending by issue number (newest first)
  return reports.sort((a, b) => b.issue - a.issue);
}

export function getReportBySlug(slug: string): { meta: ReportMeta; rawContent: string } | null {
  const contentDir = getContentDirectory();
  const possibleFiles = [
    path.join(contentDir, `${slug}.mdx`),
    path.join(contentDir, `${slug}.md`),
  ];

  let targetPath = "";
  for (const p of possibleFiles) {
    if (fs.existsSync(p)) {
      targetPath = p;
      break;
    }
  }

  if (!targetPath) {
    return null;
  }

  const fileContents = fs.readFileSync(targetPath, "utf-8");
  const { data, content } = matter(fileContents);

  const meta: ReportMeta = {
    slug,
    title: data.title || "Untitled Issue",
    issue: Number(data.issue) || 0,
    date: data.date || "",
    readTime: data.readTime || "5 min read",
    excerpt: data.excerpt || "",
    author: data.author || {
      name: "Auvis Core Team",
      role: "Engineering Dispatch",
      avatar: "/auvis-final.svg",
    },
    tags: Array.isArray(data.tags) ? data.tags : [],
    highlights: Array.isArray(data.highlights) ? data.highlights : [],
    featured: Boolean(data.featured),
  };

  return { meta, rawContent: content };
}

