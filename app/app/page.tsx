import React from "react";
import Navbar from "./components/Navbar";
import Hero from "./components/Hero";
import BentoCard from "./components/BentoCard";
import Marquee from "./components/Marquee";
import RevealInfo from "./components/RevealInfo";
import Features from "./components/Features";
import HowItWorks from "./components/HowItWorks";
import Faqs from "./components/Faqs";
import Footer from "./components/Footer";
import CtaCard from "./components/CtaCard";

const page = () => {
  return (
    <div className="flex flex-col min-h-screen">
      <Navbar />
      <Hero />
      <Marquee />
      <BentoCard />
      <RevealInfo />
      <Features />
      <HowItWorks />
      <CtaCard />
      <Faqs />
      <Footer />
    </div>
  );
};

export default page;
