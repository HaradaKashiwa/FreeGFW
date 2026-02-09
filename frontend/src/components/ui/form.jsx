import { Button } from "@/components/ui/button"
import React from "react"
import { IoCheckmark } from "react-icons/io5"
import { PiSpinner } from "react-icons/pi"

export function Form({ onSubmit, fields = [], submitLoading = false, submitText, errors }) {
    return (
        <form className='mt-4' onSubmit={e => {
            e.preventDefault()
            const formData = new FormData(e.target);
            const values = Object.fromEntries(formData.entries());
            onSubmit(values)
        }}>
            {fields?.map(field => {
                const error = errors?.find(error => error.field === field.name)
                if (error) {
                    field.component = React.cloneElement(field.component, {
                      className: 'border-red-500'
                    });
                }
                return (
                    <div key={field.name} className='mb-4 grid space-y-2'>
                        <label htmlFor={field.name}>{field.label}</label>
                        {field.component}
                        {error ? <span className="text-sm mt-1 opacity-70 text-red-500">{error.message}</span> : (
                            field.description && <span className="text-sm mt-1 opacity-70">{field.description}</span>
                        )}
                    </div>
                )
            })}
            <div className='flex justify-end'>
                <Button className='cursor-pointer' type="submit" disabled={submitLoading}>{submitText} {submitLoading ? <PiSpinner className="animate-spin" /> : <IoCheckmark />}</Button>
            </div>
        </form>
    )
}