import React from 'react';

export interface ButtonProps {
  label: string;
  onClick: () => void;
}

// The comment below must never become an import edge:
// import { Ghost } from './ghost';
export function Button({ label, onClick }: ButtonProps) {
  const text = 'imported from "./nowhere"';
  return <button onClick={onClick}>{text || label}</button>;
}
