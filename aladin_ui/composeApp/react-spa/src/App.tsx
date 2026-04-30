import {MilkdownEditor} from "./MilkdownEditor";
import {Bridge} from "./embed";
import {useEffect} from "react";

export type ArtifactSpaProps = {
    widgetId?: string,
    overlayRoot?: HTMLElement | null,
    bridge: Bridge | undefined,
};

export default function App({
                                bridge,
                                overlayRoot = null,
                            }: ArtifactSpaProps) {

    useEffect(() => {
        if (!bridge) return;
        bridge.kotlinEvent = (event) => {
            console.log("JS event received: ", event);
        }
        return () => {
            bridge.kotlinEvent = (event) => {
                console.log(`JS event received: ${event} | NO_OP event handler!`);
            }
        }
    }, [bridge]);


    return (
        <>
            <div className="app-shell" style={{
                height: "100%",
                boxSizing: "border-box",
            }}>
                <div style={
                    {
                        height: "100%",
                        paddingLeft: "15px",
                        paddingRight: "15px",
                        boxSizing: "border-box",
                        overflowY: "auto",
                    }
                }>
                    <MilkdownEditor/>
                </div>
            </div>
        </>
    );
}