import React from "react";
import Navbar from "../components/Navbar";
import Footer from "../components/Footer";
import { getDocsSections } from "@/lib/docs";
import { DocsSidebar } from "./components/DocsSidebar";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const sections = getDocsSections();

  return (
    <div className="flex flex-col min-h-screen bg-white">
      <Navbar />

      <div className="flex-1 pt-16">
        <div className="mx-auto max-w-[1400px] px-4 sm:px-6 lg:px-8">
          <div className="flex">
            {/* Left Sticky Sidebar (Desktop) */}
            <div className="hidden lg:block w-64 shrink-0 border-r border-slate-200/80 pr-6 pt-10 sticky top-16 h-[calc(100vh-4rem)] overflow-y-auto">
              <DocsSidebar sections={sections} />
            </div>

            {/* Main Content Area */}
            <div className="flex-1 min-w-0 pt-8 pb-16 lg:pl-10 lg:pr-8">
              {children}
            </div>
          </div>
        </div>
      </div>

      <Footer />
    </div>
  );
}

