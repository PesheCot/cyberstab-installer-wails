import React, { useEffect, useRef, useState } from "react";

type AnimatedStepProps = {
  active: boolean;
  children: React.ReactNode;
  direction?: "forward" | "backward";
};

/**
 * AnimatedStep — wrapper that applies enter/exit animations
 * when step changes in the wizard.
 */
export const AnimatedStep: React.FC<AnimatedStepProps> = ({
  active,
  children,
  direction = "forward",
}) => {
  const [shouldRender, setShouldRender] = useState(active);
  const [animationClass, setAnimationClass] = useState("");
  const timeoutRef = useRef<number | null>(null);

  useEffect(() => {
    if (active) {
      setShouldRender(true);
      // Start enter animation
      setAnimationClass(
        direction === "forward" ? "slideForwardEnter" : "slideBackwardEnter"
      );
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    } else if (shouldRender) {
      // Start exit animation
      setAnimationClass("fadeExit");
      timeoutRef.current = window.setTimeout(() => {
        setShouldRender(false);
      }, 300);
    }

    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, [active, direction, shouldRender]);

  if (!shouldRender) return null;

  return (
    <div className={`animatedStep ${animationClass}`}>
      {children}
    </div>
  );
};
