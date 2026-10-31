import fs from "fs";
import path from "path";
import matter from "gray-matter";

export interface DocMeta {
  slug: string;
  title: string;
  description: string;
  section: string;
  order: number;
}

export interface TOCItem {
  id: string;
  text: string;
  level: number;
}

export interface DocSection {
  title: string;
  items: {
    slug: string;
    title: string;
    order: number;
  }[];
}

function getDocsDirectory(): string {
  const candidates = [
    path.join(process.cwd(), "content/docs"),
    path.join(process.cwd(), "app/content/docs"),
  ];

  for (const dir of candidates) {
    if (fs.existsSync(dir)) {
      return dir;
    }
  }

  return candidates[0];
}

export function getAllDocs(): DocMeta[] {
  const docsDir = getDocsDirectory();

  if (!fs.existsSync(docsDir)) {
    return [];
  }

  const files = fs.readdirSync(docsDir).filter((f) => f.endsWith(".mdx") || f.endsWith(".md"));

  const docs: DocMeta[] = files.map((fileName) => {
    const slug = fileName.replace(/\.mdx?$/, "");
    const fullPath = path.join(docsDir, fileName);
    const fileContents = fs.readFileSync(fullPath, "utf-8");
    const { data } = matter(fileContents.trimStart());

    return {
      slug,
      title: data.title || slug,
      description: data.description || "",
      section: data.section || "General",
      order: typeof data.order === "number" ? data.order : 99,
    };
  });

  return docs;
}

export function getDocsSections(): DocSection[] {
  const docs = getAllDocs();
  const sectionMap = new Map<string, DocMeta[]>();

  for (const doc of docs) {
    if (!sectionMap.has(doc.section)) {
      sectionMap.set(doc.section, []);
    }
    sectionMap.get(doc.section)!.push(doc);
  }

  // Predefined section display order
  const sectionPriority: Record<string, number> = {
    "Getting Started": 1,
    "Configuration & Routing": 2,
    "Policies": 3,
    "Policy Engine & DSL": 3,
    "Streaming & Ingestion": 4,
    "Observability & Security": 5,
  };

  const sections: DocSection[] = [];

  for (const [title, items] of sectionMap.entries()) {
    items.sort((a, b) => a.order - b.order);
    sections.push({
      title,
      items: items.map((i) => ({ slug: i.slug, title: i.title, order: i.order })),
    });
  }

  sections.sort((a, b) => {
    const orderA = sectionPriority[a.title] || 99;
    const orderB = sectionPriority[b.title] || 99;
    return orderA - orderB;
  });

  return sections;
}

export function extractTOC(content: string): TOCItem[] {
  const headingRegex = /^(#{2,3})\s+(.*)$/gm;
  const toc: TOCItem[] = [];
  let match;

  while ((match = headingRegex.exec(content)) !== null) {
    const level = match[1].length;
    const rawText = match[2].trim();
    // Strip markdown formatting like bold, code, links
    const text = rawText
      .replace(/`([^`]+)`/g, "$1")
      .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
      .replace(/\*\*([^*]+)\*\*/g, "$1")
      .replace(/\*([^*]+)\*/g, "$1");

    const id = text
      .toLowerCase()
      .replace(/[^\w\s-]/g, "")
      .replace(/\s+/g, "-");

    toc.push({ id, text, level });
  }

  return toc;
}

export function getDocBySlug(slug: string): { meta: DocMeta; rawContent: string; toc: TOCItem[] } | null {
  const docsDir = getDocsDirectory();
  const possiblePaths = [
    path.join(docsDir, `${slug}.mdx`),
    path.join(docsDir, `${slug}.md`),
  ];

  let targetPath = "";
  for (const p of possiblePaths) {
    if (fs.existsSync(p)) {
      targetPath = p;
      break;
    }
  }

  if (!targetPath) {
    return null;
  }

  const fileContents = fs.readFileSync(targetPath, "utf-8");
  const { data, content } = matter(fileContents.trimStart());
  const toc = extractTOC(content);

  const meta: DocMeta = {
    slug,
    title: data.title || slug,
    description: data.description || "",
    section: data.section || "General",
    order: typeof data.order === "number" ? data.order : 99,
  };

  return { meta, rawContent: content, toc };
}

