import React, { useEffect, useState } from "react"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
  } from "@/components/ui/dialog"

export function Modal({ children, title, description, content, open, onOpenChange }) {
    const [innerOpen, setInnerOpen] = useState(false)
    return (
        <>
            {open !== undefined ? children : React.cloneElement(children, { onClick: () => setInnerOpen(true) })}
            {(!!open || innerOpen) && (
                <Dialog open onOpenChange={onOpenChange ? onOpenChange : setInnerOpen}>
                    <DialogContent className='@container/modal'>
                        <DialogHeader>
                            <DialogTitle>{title}</DialogTitle>
                            <DialogDescription>
                                {description}
                            </DialogDescription>
                        </DialogHeader>
                        {content}
                    </DialogContent>
                </Dialog>
            )}
        </>
    )
}