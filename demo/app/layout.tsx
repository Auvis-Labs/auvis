import './globals.css';
import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'AUVIS // Real-Time Data Flow Visualizer',
  description: 'Live Interactive Sine Wave Data Governance Pipeline Visualizer',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="dark">
      <body className="bg-[#07090e] text-slate-100 antialiased selection:bg-indigo-500/30 selection:text-indigo-200">
        {children}
      </body>
    </html>
  );
}
