/// <reference types="svelte" />
/// <reference types="vite/client" />

interface Window {
  go?: {
    main: {
      App: {
        GetVersion(): Promise<string>;
        GetTransferSettings(): Promise<any>;
        SaveTransferSettings(settings: any): Promise<string>;
        [key: string]: (...args: any[]) => Promise<any>;
      };
    };
  };
}
